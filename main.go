package main

import (
	"os"
	"os/exec"
	"time"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util"
	k8sexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/node"
)

var (
	flags = flag.NewFlagSet("", flag.ContinueOnError)

	cluster = flags.Bool("use-kubernetes-cluster-service", true, `If true, use the built in kubernetes
        cluster for creating the client`)

	verbose = flags.Bool("v", false, `enable verbose output`)
)

func main() {
	clientConfig := kubectl_util.DefaultClientConfig(flags)
	flags.Parse(os.Args)

	var kubeClient *unversioned.Client
	var err error

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
		glog.Fatalf("Error loading ip_vs module: %v", err)
	}

	ipvsc := newIPVSController(kubeClient, namespace)
	go ipvsc.epController.Run(util.NeverStop)
	go ipvsc.svcController.Run(util.NeverStop)
	util.Until(ipvsc.worker, time.Second, util.NeverStop)

	// if the cluster has more than one node we start ExaBGP to announce
	// periodically that the VIP/s are running in this node
	// TODO: use watch to avoid kill the node.
	clusterNodes := []string{}
	nodes, err := kubeClient.Nodes().List(api.ListOptions{})
	for _, nodo := range nodes.Items {
		nodeIP, err := node.GetNodeHostIP(&nodo)
		if err == nil {
			clusterNodes = append(clusterNodes, nodeIP.String())
		}
	}

	if len(clusterNodes) > 1 {
		glog.Info("starting BGP server to announce VIPs")

		ip, err := myIP()
		if err != nil {
			glog.Fatalf("Error creating exabgp files: %v", err)
		}

		err = writeBGPCfg(ip, clusterNodes)
		if err != nil {
			glog.Fatalf("Error creating exabgp files: %v", err)
		}

		err = writeHealthcheck(ipvsc.getVIPs())
		if err != nil {
			glog.Fatalf("Error creating exabgp files: %v", err)
		}

		startBGPServer()
	}
}

func startBGPServer() {
	cmd := exec.Command("exabgp", "/etc/exabgp/exabgp.conf")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		glog.Errorf("exabgp error: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		glog.Fatalf("exabgp error: %v", err)
	}
}

func loadIPVModule() error {
	out, err := k8sexec.New().Command("modprobe", "ip_vs").CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Error loading ip_vip: %s, %v", string(out), err)
		return err
	}

	_, err = os.Stat("/proc/net/ip_vs")
	if err != nil {
		return err
	}

	return nil
}
