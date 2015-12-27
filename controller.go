package main

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"
	"github.com/qmsk/clusterf"
	clusterf_cfg "github.com/qmsk/clusterf/config"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/util/workqueue"
)

const (
	reloadQPS     = 10.0
	resyncPeriod  = 10 * time.Second
	ipvsPublicVIP = "k8s.io/public-vip"
)

var (
	// keyFunc for endpoints and services.
	keyFunc = framework.DeletionHandlingMetaNamespaceKeyFunc

	// Error used to indicate that a sync is deferred because the controller isn't ready yet
	errDeferredSync = fmt.Errorf("deferring sync till endpoints controller has synced")
)

// ipvsControllerController watches the kubernetes api and adds/removes
// services from LVS throgh ipvsadmin.
type ipvsControllerController struct {
	queue             *workqueue.Type
	client            *unversioned.Client
	epController      *framework.Controller
	svcController     *framework.Controller
	svcLister         cache.StoreToServiceLister
	epLister          cache.StoreToEndpointsLister
	reloadRateLimiter util.RateLimiter
	ipvsConfig        clusterf.IpvsConfig
}

// getEndpoints returns a list of <endpoint ip>:<port> for a given service/target port combination.
func (ipvsc *ipvsControllerController) getEndpoints(
	s *api.Service, servicePort *api.ServicePort) (endpoints []*clusterf_cfg.ConfigServiceBackend) {
	ep, err := ipvsc.epLister.GetServiceEndpoints(s)
	if err != nil {
		return
	}

	// The intent here is to create a union of all subsets that match a targetPort.
	// We know the endpoint already matches the service, so all pod ips that have
	// the target port are capable of service traffic for it.
	for _, ss := range ep.Subsets {
		for _, epPort := range ss.Ports {
			var targetPort uint16
			switch servicePort.TargetPort.Type {
			case intstr.Int:
				if epPort.Port == servicePort.TargetPort.IntValue() {
					targetPort = uint16(epPort.Port)
				}
			case intstr.String:
				if epPort.Name == servicePort.TargetPort.StrVal {
					targetPort = uint16(epPort.Port)
				}
			}
			if targetPort == 0 {
				continue
			}
			for _, epAddress := range ss.Addresses {
				endpoints = append(endpoints, &clusterf_cfg.ConfigServiceBackend{
					ConfigSource: "k8s",
					ServiceName:  fmt.Sprintf("%v/%v", s.Namespace, s.Name),
					BackendName:  fmt.Sprintf("pod-%v", epAddress.IP),
					Backend:      clusterf_cfg.ServiceBackend{IPv4: epAddress.IP, TCP: targetPort},
				})
			}
		}
	}
	return
}

func getServiceNameForLBRule(s *api.Service, servicePort int) string {
	return fmt.Sprintf("%v:%v", s.Name, servicePort)
}

// getServices returns a list of services and their endpoints.
func (ipvsc *ipvsControllerController) getVIPs() (vips []string) {
	vips = []string{}

	services, _ := ipvsc.svcLister.List()
	for _, s := range services.Items {
		if externalIP, ok := s.GetAnnotations()[ipvsPublicVIP]; ok {
			vips = append(vips, externalIP)
		}
	}

	return
}

// getServices returns a list of services and their endpoints.
func (ipvsc *ipvsControllerController) getServices() (svcs *clusterf.Services) {
	svcs = clusterf.NewServices()

	services, _ := ipvsc.svcLister.List()
	for _, s := range services.Items {
		if externalIP, ok := s.GetAnnotations()[ipvsPublicVIP]; ok {
			svcs.NewConfig(&clusterf_cfg.ConfigService{
				ConfigSource: "k8s",
				ServiceName:  fmt.Sprintf("%v/%v", s.Namespace, s.Name),
			})

			for _, servicePort := range s.Spec.Ports {
				ep := ipvsc.getEndpoints(&s, &servicePort)
				if len(ep) == 0 {
					glog.Infof("No endpoints found for service %v, port %+v", s.Name, servicePort)
					continue
				}

				svcs.NewConfig(&clusterf_cfg.ConfigServiceFrontend{
					ConfigSource: "k8s",
					ServiceName:  fmt.Sprintf("%v/%v", s.Namespace, s.Name),
					Frontend: clusterf_cfg.ServiceFrontend{
						IPv4: externalIP,
						TCP:  uint16(servicePort.Port),
					},
				})

				for _, backend := range ep {
					svcs.NewConfig(backend)
				}

				glog.Infof("Found service: %v", s.Name)
			}
		}
	}

	return
}

// sync all services with the loadbalancer.
func (ipvsc *ipvsControllerController) sync() error {
	if !ipvsc.epController.HasSynced() || !ipvsc.svcController.HasSynced() {
		time.Sleep(100 * time.Millisecond)
		return errDeferredSync
	}

	svcs := ipvsc.getServices()
	_, err := svcs.SyncIPVS(ipvsc.ipvsConfig)
	if err != nil {
		return err
	}

	ipvsc.reloadRateLimiter.Accept()
	return nil
}

// worker handles the work queue.
func (ipvsc *ipvsControllerController) worker() {
	for {
		key, _ := ipvsc.queue.Get()
		glog.Infof("Sync triggered by service %v", key)
		if err := ipvsc.sync(); err != nil {
			glog.Infof("Requeuing %v because of error: %v", key, err)
			ipvsc.queue.Add(key)
		}
		ipvsc.queue.Done(key)
	}
}

// newIPVSController creates a new controller from the given config.
func newIPVSController(kubeClient *unversioned.Client, namespace string) *ipvsControllerController {
	ipvsc := ipvsControllerController{
		client: kubeClient,
		queue:  workqueue.New(),
		reloadRateLimiter: util.NewTokenBucketRateLimiter(
			reloadQPS, int(reloadQPS)),
		ipvsConfig: clusterf.IpvsConfig{
			FwdMethod: "masq", // droute
			SchedName: "wlc",
		},
	}

	enqueue := func(obj interface{}) {
		key, err := keyFunc(obj)
		if err != nil {
			glog.Infof("Couldn't get key for object %+v: %v", obj, err)
			return
		}

		ipvsc.queue.Add(key)
	}

	eventHandlers := framework.ResourceEventHandlerFuncs{
		AddFunc:    enqueue,
		DeleteFunc: enqueue,
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				enqueue(cur)
			}
		},
	}

	ipvsc.svcLister.Store, ipvsc.svcController = framework.NewInformer(
		cache.NewListWatchFromClient(
			ipvsc.client, "services", namespace, fields.Everything()),
		&api.Service{}, resyncPeriod, eventHandlers)

	ipvsc.epLister.Store, ipvsc.epController = framework.NewInformer(
		cache.NewListWatchFromClient(
			ipvsc.client, "endpoints", namespace, fields.Everything()),
		&api.Endpoints{}, resyncPeriod, eventHandlers)

	return &ipvsc
}
