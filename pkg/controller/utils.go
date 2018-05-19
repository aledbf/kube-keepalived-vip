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
	"errors"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/aledbf/kube-keepalived-vip/pkg/k8s"
)

var (
	invalidIfaces = []string{"lo", "docker0", "flannel.1", "cbr0"}
	nsSvcLbRegex  = regexp.MustCompile(`(.*)/(.*):(.*)|(.*)/(.*)`)
	vethRegex     = regexp.MustCompile(`^veth.*`)
	caliRegex     = regexp.MustCompile(`^cali.*`)
	lvsRegex      = regexp.MustCompile(`NAT|DR|PROXY`)
)

type nodeInfo struct {
	iface   string
	ip      string
	netmask int
}

// getNetworkInfo returns information of the node where the pod is running
func getNetworkInfo(ip string) (*nodeInfo, error) {
	iface, _, mask := interfaceByIP(ip)
	return &nodeInfo{
		iface:   iface,
		ip:      ip,
		netmask: mask,
	}, nil
}

// netInterfaces returns a slice containing the local network interfaces
// excluding lo, docker0, flannel.1 and veth interfaces.
func netInterfaces() []net.Interface {
	validIfaces := []net.Interface{}
	ifaces, err := net.Interfaces()
	if err != nil {
		glog.Errorf("unexpected error obtaining network interfaces: %v", err)
		return validIfaces
	}

	for _, iface := range ifaces {
		if !vethRegex.MatchString(iface.Name) &&
			!caliRegex.MatchString(iface.Name) &&
			stringSlice(invalidIfaces).pos(iface.Name) == -1 {
			validIfaces = append(validIfaces, iface)
		}
	}

	glog.V(2).Infof("network interfaces: %+v", validIfaces)
	return validIfaces
}

// interfaceByIP returns the local network interface name that is using the
// specified IP address. If no interface is found returns an empty string.
func interfaceByIP(ip string) (string, string, int) {
	for _, iface := range netInterfaces() {
		ifaceIP, mask, err := ipByInterface(iface.Name)
		if err == nil && ip == ifaceIP {
			return iface.Name, ip, mask
		}
	}

	glog.Warningf("no interface with IP address %v detected in the node", ip)
	return "", "", 0
}

func ipByInterface(name string) (string, int, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", 32, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", 32, err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				ones, _ := ipnet.Mask.Size()
				mask := ones
				return ip, mask, nil
			}
		}
	}

	return "", 32, errors.New("found no IPv4 addresses.")
}

type stringSlice []string

// pos returns the position of a string in a slice.
// If it does not exists in the slice returns -1.
func (slice stringSlice) pos(value string) int {
	for p, v := range slice {
		if v == value {
			return p
		}
	}

	return -1
}

// getClusterNodesIP returns the IP address of each node in the kubernetes cluster
func getClusterNodesIP(kubeClient *kubernetes.Clientset, nodeSelector string) (clusterNodes []string) {
	listOpts := metav1.ListOptions{}

	if nodeSelector != "" {
		label, err := labels.Parse(nodeSelector)
		if err != nil {
			glog.Fatalf("'%v' is not a valid selector: %v", nodeSelector, err)
		}
		listOpts.LabelSelector = label.String()
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(listOpts)
	if err != nil {
		glog.Fatalf("Error getting running nodes: %v", err)
	}

	for _, nodo := range nodes.Items {
		nodeIP := k8s.GetNodeIP(kubeClient, nodo.Name)
		clusterNodes = append(clusterNodes, nodeIP)
	}
	sort.Strings(clusterNodes)

	return
}

// getNodeNeighbors returns a list of IP address of the nodes
func getNodeNeighbors(nodeInfo *nodeInfo, clusterNodes []string) (neighbors []string) {
	for _, neighbor := range clusterNodes {
		if nodeInfo.ip != neighbor {
			neighbors = append(neighbors, neighbor)
		}
	}
	sort.Strings(neighbors)
	return
}

// getPriority returns the priority of one node using the
// IP address as key. It starts in 100
func getNodePriority(ip string, nodes []string) int {
	return 100 + stringSlice(nodes).pos(ip)
}

func appendIfMissing(slice []string, item string) []string {
	for _, elem := range slice {
		if elem == item {
			return slice
		}
	}
	return append(slice, item)
}

func parseNsName(input string) (string, string, error) {
	nsName := strings.Split(input, "/")
	if len(nsName) != 2 {
		return "", "", fmt.Errorf("invalid format (namespace/name) found in '%v'", input)
	}

	return nsName[0], nsName[1], nil
}

func parseNsSvcLVS(input string) (string, string, string, error) {
	nsSvcLb := nsSvcLbRegex.FindStringSubmatch(input)
	if len(nsSvcLb) != 6 {
		return "", "", "", fmt.Errorf("invalid format (namespace/service name[:NAT|DR|PROXY]) found in '%v'", input)
	}

	ns := nsSvcLb[1]
	svc := nsSvcLb[2]
	kind := nsSvcLb[3]

	if ns == "" {
		ns = nsSvcLb[4]
	}

	if svc == "" {
		svc = nsSvcLb[5]
	}

	if kind == "" {
		kind = "NAT"
	}

	if !lvsRegex.MatchString(kind) {
		return "", "", "", fmt.Errorf("invalid LVS method. Only NAT,DR and PROXY are supported: %v", kind)
	}

	return ns, svc, kind, nil
}

type nodeSelector map[string]string

func (ns nodeSelector) String() string {
	kv := []string{}
	for key, val := range ns {
		kv = append(kv, fmt.Sprintf("%v=%v", key, val))
	}

	return strings.Join(kv, ",")
}

func parseNodeSelector(data map[string]string) string {
	return nodeSelector(data).String()
}
