package main

import (
	"os"
	"time"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/util/proc"
	"k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util"
)

var (
	flags = flag.NewFlagSet("", flag.ContinueOnError)

	cluster = flags.Bool("use-kubernetes-cluster-service", true, `If true, use the built in kubernetes
        cluster for creating the client`)

	logLevel = flags.Int("v", 1, `verbose output`)

	// sysctl changes required by keepalived
	sysctlAdjustments = map[string]int{
		"net/ipv4/ip_nonlocal_bind": 1,
	}
)

func main() {
	clientConfig := kubectl_util.DefaultClientConfig(flags)
	flags.Parse(os.Args)

	var err error
	var kubeClient *unversioned.Client

	if *cluster {
		if kubeClient, err = unversioned.NewInCluster(); err != nil {
			glog.Fatalf("Failed to create client: %v", err)
		}
	} else {
		config, err := clientConfig.ClientConfig()
		if err != nil {
			glog.Fatalf("error connecting to the client: %v", err)
		}
		kubeClient, err = unversioned.New(config)
	}

	namespace, specified, err := clientConfig.Namespace()
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}

	if !specified {
		namespace = ""
	}

	err = loadIPVModule()
	if err != nil {
		glog.Fatalf("Terminating execution: %v", err)
	}

	err = changeSysctl()
	if err != nil {
		glog.Fatalf("Terminating execution: %v", err)
	}

	proc.StartReaper()

	glog.Info("starting LVS configuration")
	ipvsc := newIPVSController(kubeClient, namespace)
	go ipvsc.epController.Run(util.NeverStop)
	go ipvsc.svcController.Run(util.NeverStop)
	go util.Until(ipvsc.worker, time.Second, util.NeverStop)

	time.Sleep(5 * time.Second)
	glog.Info("starting keepalived to announce VIPs")
	ipvsc.keepalived.Start()
}
