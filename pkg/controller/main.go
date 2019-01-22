/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/flowcontrol"

	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	utilexec "k8s.io/utils/exec"

	"github.com/aledbf/kube-keepalived-vip/pkg/k8s"
	"github.com/aledbf/kube-keepalived-vip/pkg/store"
	"github.com/aledbf/kube-keepalived-vip/pkg/task"
)

const (
	resyncPeriod = 0
)

type service struct {
	IP   string
	Port int
}

type serviceByIPPort []service

func (c serviceByIPPort) Len() int      { return len(c) }
func (c serviceByIPPort) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c serviceByIPPort) Less(i, j int) bool {
	iIP := c[i].IP
	jIP := c[j].IP
	if iIP != jIP {
		return iIP < jIP
	}

	iPort := c[i].Port
	jPort := c[j].Port
	return iPort < jPort
}

type vip struct {
	Name      string
	IP        string
	Port      int
	Protocol  string
	LVSMethod string
	iface     string
	Backends  []service
}

type vipByNameIPPort []vip

func (c vipByNameIPPort) Len() int      { return len(c) }
func (c vipByNameIPPort) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c vipByNameIPPort) Less(i, j int) bool {
	iName := c[i].Name
	jName := c[j].Name
	if iName != jName {
		return iName < jName
	}

	iIP := c[i].IP
	jIP := c[j].IP
	if iIP != jIP {
		return iIP < jIP
	}

	iPort := c[i].Port
	jPort := c[j].Port
	return iPort < jPort
}

// ipvsControllerController watches the kubernetes api and adds/removes
// services from LVS throgh ipvsadmin.
type ipvsControllerController struct {
	client *kubernetes.Clientset

	epController  cache.Controller
	mapController cache.Controller
	svcController cache.Controller

	svcLister store.ServiceLister
	epLister  store.EndpointLister
	mapLister store.ConfigMapLister

	reloadRateLimiter flowcontrol.RateLimiter

	keepalived *keepalived

	defaultIface string

	configMapName            string
	configMapResourceVersion string

	httpPort int

	ruMD5 string

	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock sync.Mutex

	shutdown bool

	syncQueue *task.Queue

	stopCh chan struct{}
}

// getEndpoints returns a list of <endpoint ip>:<port> for a given service/target port combination.
func (ipvsc *ipvsControllerController) getEndpoints(
	s *apiv1.Service, servicePort *apiv1.ServicePort) []service {
	ep, err := ipvsc.epLister.GetServiceEndpoints(s)
	if err != nil {
		glog.Warningf("unexpected error getting service endpoints: %v", err)
		return []service{}
	}

	var endpoints []service

	// The intent here is to create a union of all subsets that match a targetPort.
	// We know the endpoint already matches the service, so all pod ips that have
	// the target port are capable of service traffic for it.
	for _, ss := range ep.Subsets {
		for _, epPort := range ss.Ports {
			var targetPort int
			switch servicePort.TargetPort.Type {
			case intstr.Int:
				if int(epPort.Port) == servicePort.TargetPort.IntValue() {
					targetPort = int(epPort.Port)
				}
			case intstr.String:
				if epPort.Name == servicePort.TargetPort.StrVal {
					targetPort = int(epPort.Port)
				}
			}
			if targetPort == 0 {
				continue
			}
			for _, epAddress := range ss.Addresses {
				endpoints = append(endpoints, service{IP: epAddress.IP, Port: targetPort})
			}
		}
	}

	return endpoints
}

// getServices returns a list of services and their endpoints.
func (ipvsc *ipvsControllerController) getServices(cfgMap *apiv1.ConfigMap) []vip {
	svcs := []vip{}

	// k -> OPTIONAL_INDEX-IP_ADDRESS@OPTIONAL_INTERFACE, regexp: (\w+-)?([a-f\d.:]+)(@.+)?;
	// v -> <namespace>/<service name>:<lvs method>
	for address, nsSvcLvs := range cfgMap.Data {
		externalIP, iface, err := parseAddress(address)
		if err != nil {
			glog.Warningf("%v", err)
			continue
		}

		if iface == "" {
			iface = ipvsc.defaultIface
		}

		if nsSvcLvs == "" {
			// if target is empty string we will not forward to any service but
			// instead just configure the IP on the machine and let it up to
			// another Pod or daemon to bind to the IP address
			svcs = append(svcs, vip{
				Name:      "",
				IP:        externalIP,
				Port:      0,
				LVSMethod: "VIP",
				Backends:  nil,
				Protocol:  "TCP",
				iface:     iface,
			})
			glog.V(2).Infof("Adding VIP only service: %v", externalIP)
			continue
		}

		ns, svc, lvsm, err := parseNsSvcLVS(nsSvcLvs)
		if err != nil {
			glog.Warningf("%v", err)
			continue
		}

		nsSvc := fmt.Sprintf("%v/%v", ns, svc)
		svcObj, svcExists, err := ipvsc.svcLister.Store.GetByKey(nsSvc)
		if err != nil {
			glog.Warningf("error getting service %v: %v", nsSvc, err)
			continue
		}

		if !svcExists {
			glog.Warningf("service %v not found", nsSvc)
			continue
		}

		s := svcObj.(*apiv1.Service)
		for _, servicePort := range s.Spec.Ports {
			ep := ipvsc.getEndpoints(s, &servicePort)
			if len(ep) == 0 {
				glog.Warningf("no endpoints found for service %v, port %+v", s.Name, servicePort)
				continue
			}

			sort.Sort(serviceByIPPort(ep))

			svcs = append(svcs, vip{
				Name:      fmt.Sprintf("%v-%v", s.Namespace, s.Name),
				IP:        externalIP,
				Port:      int(servicePort.Port),
				LVSMethod: lvsm,
				Backends:  ep,
				Protocol:  fmt.Sprintf("%v", servicePort.Protocol),
				iface:     iface,
			})
			glog.V(2).Infof("found service: %v:%v", s.Name, servicePort.Port)
		}
	}

	sort.Sort(vipByNameIPPort(svcs))

	return svcs
}

// sync all services with the
func (ipvsc *ipvsControllerController) sync(key interface{}) error {
	ipvsc.reloadRateLimiter.Accept()

	ns, name, err := parseNsName(ipvsc.configMapName)
	if err != nil {
		glog.Warningf("%v", err)
		return err
	}
	cfgMap, err := ipvsc.getConfigMap(ns, name)
	if err != nil {
		return fmt.Errorf("unexpected error searching configmap %v: %v", ipvsc.configMapName, err)
	}

	if ipvsc.configMapResourceVersion == cfgMap.ObjectMeta.ResourceVersion {
		glog.V(2).Infof("No change to %s ConfigMap", name)
		return nil
	}

	ipvsc.configMapResourceVersion = cfgMap.ObjectMeta.ResourceVersion
	svc := ipvsc.getServices(cfgMap)

	err = ipvsc.keepalived.WriteCfg(svc)
	if err != nil {
		return err
	}

	glog.V(2).Infof("services: %v", svc)

	md5, err := checksum(keepalivedCfg)
	if err == nil && md5 == ipvsc.ruMD5 {
		return nil
	}

	ipvsc.ruMD5 = md5
	err = ipvsc.keepalived.Reload()
	if err != nil {
		glog.Errorf("error reloading keepalived: %v", err)
	}

	return nil
}

// Stop stops the loadbalancer controller.
func (ipvsc *ipvsControllerController) Start() {
	go ipvsc.epController.Run(ipvsc.stopCh)
	go ipvsc.svcController.Run(ipvsc.stopCh)
	go ipvsc.mapController.Run(ipvsc.stopCh)

	go ipvsc.syncQueue.Run(time.Second, ipvsc.stopCh)

	go handleSigterm(ipvsc)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(ipvsc.stopCh,
		ipvsc.epController.HasSynced,
		ipvsc.svcController.HasSynced,
		ipvsc.mapController.HasSynced,
	) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
	}

	go func() {
		glog.Infof("Starting HTTP server on port %d", ipvsc.httpPort)
		err := http.ListenAndServe(fmt.Sprintf(":%d", ipvsc.httpPort), nil)
		if err != nil {
			glog.Error(err.Error())
		}
	}()

	glog.Info("starting keepalived to announce VIPs")
	ipvsc.keepalived.Start()
}

func handleSigterm(ipvsc *ipvsControllerController) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	glog.Infof("Received SIGTERM, shutting down")

	exitCode := 0
	if err := ipvsc.Stop(); err != nil {
		glog.Infof("Error during shutdown %v", err)
		exitCode = 1
	}

	glog.Infof("Exiting with %v", exitCode)
	os.Exit(exitCode)
}

// Stop stops the loadbalancer controller.
func (ipvsc *ipvsControllerController) Stop() error {
	ipvsc.stopLock.Lock()
	defer ipvsc.stopLock.Unlock()

	if !ipvsc.syncQueue.IsShuttingDown() {
		glog.Infof("shutting down controller queues")
		close(ipvsc.stopCh)
		go ipvsc.syncQueue.Shutdown()

		ipvsc.keepalived.Stop()

		return nil
	}

	return fmt.Errorf("shutdown already in progress")
}

// NewIPVSController creates a new controller from the given config.
func NewIPVSController(kubeClient *kubernetes.Clientset, namespace string, useUnicast bool, configMapName string, vrid int, proxyMode bool, iface string, httpPort int) *ipvsControllerController {
	podInfo, err := k8s.GetPodDetails(kubeClient)
	if err != nil {
		glog.Fatalf("Error getting POD information: %v", err)
	}

	pod, err := kubeClient.CoreV1().Pods(podInfo.Namespace).Get(podInfo.Name, metav1.GetOptions{})
	if err != nil {
		glog.Fatalf("Error getting %v: %v", podInfo.Name, err)
	}

	selector := parseNodeSelector(pod.Spec.NodeSelector)
	clusterNodes := getClusterNodesIP(kubeClient, selector)

	nodeInfo, err := getNetworkInfo(podInfo.NodeIP)
	if err != nil {
		glog.Fatalf("Error getting local IP from nodes in the cluster: %v", err)
	}
	neighbors := getNodeNeighbors(nodeInfo, clusterNodes)

	if iface == "" {
		iface = nodeInfo.iface
		glog.Info("No interface was provided, proceeding with the node's default: ", iface)
	}
	execer := utilexec.New()
	dbus := utildbus.New()
	iptInterface := utiliptables.New(execer, dbus, utiliptables.ProtocolIpv4)

	ipvsc := ipvsControllerController{
		client:            kubeClient,
		reloadRateLimiter: flowcontrol.NewTokenBucketRateLimiter(0.5, 1),
		defaultIface:      iface,
		configMapName:     configMapName,
		httpPort:          httpPort,
		stopCh:            make(chan struct{}),
	}

	ipvsc.keepalived = &keepalived{
		iface:      iface,
		ip:         nodeInfo.ip,
		netmask:    nodeInfo.netmask,
		nodes:      clusterNodes,
		neighbors:  neighbors,
		priority:   getNodePriority(nodeInfo.ip, clusterNodes),
		useUnicast: useUnicast,
		ipt:        iptInterface,
		vrid:       vrid,
		proxyMode:  proxyMode,
	}

	ipvsc.syncQueue = task.NewTaskQueue(ipvsc.sync)

	err = ipvsc.keepalived.loadTemplates()
	if err != nil {
		glog.Fatalf("Error loading templates: %v", err)
	}

	mapEventHandler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				upCmap := cur.(*apiv1.ConfigMap)
				mapKey := fmt.Sprintf("%s/%s", upCmap.Namespace, upCmap.Name)
				// updates to configuration configmaps can trigger an update
				if mapKey == ipvsc.configMapName {
					ipvsc.syncQueue.Enqueue(cur)
				}
			}
		},
	}

	eventHandlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ipvsc.syncQueue.Enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			ipvsc.syncQueue.Enqueue(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				ipvsc.syncQueue.Enqueue(cur)
			}
		},
	}

	ipvsc.svcLister.Store, ipvsc.svcController = cache.NewInformer(
		cache.NewListWatchFromClient(ipvsc.client.CoreV1().RESTClient(), "services", namespace, fields.Everything()),
		&apiv1.Service{}, resyncPeriod, cache.ResourceEventHandlerFuncs{})

	ipvsc.epLister.Store, ipvsc.epController = cache.NewInformer(
		cache.NewListWatchFromClient(ipvsc.client.CoreV1().RESTClient(), "endpoints", namespace, fields.Everything()),
		&apiv1.Endpoints{}, resyncPeriod, eventHandlers)

	ipvsc.mapLister.Store, ipvsc.mapController = cache.NewInformer(
		cache.NewListWatchFromClient(ipvsc.client.CoreV1().RESTClient(), "configmaps", namespace, fields.Everything()),
		&apiv1.ConfigMap{}, resyncPeriod, mapEventHandler)

	http.HandleFunc("/health", func(rw http.ResponseWriter, req *http.Request) {
		err := ipvsc.keepalived.Healthy()
		if err != nil {
			glog.Errorf("Health check unsuccessful: %v", err)
			http.Error(rw, fmt.Sprintf("keepalived not healthy: %v", err), 500)
			return
		}

		glog.V(3).Info("Health check successful")
		fmt.Fprint(rw, "OK")
	})

	return &ipvsc
}

func (ipvsc *ipvsControllerController) getConfigMap(ns, name string) (*apiv1.ConfigMap, error) {
	s, exists, err := ipvsc.mapLister.Store.GetByKey(fmt.Sprintf("%v/%v", ns, name))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("configmap %v was not found", name)
	}
	return s.(*apiv1.ConfigMap), nil
}

func checksum(filename string) (string, error) {
	var result []byte
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(result)), nil
}
