package main

import (
	"errors"
	"net"
	"regexp"

	"github.com/golang/glog"
)

const (
	vethRegex = "^veth.*"
)

var (
	invalidIfaces = []string{"lo", "docker0", "flannel.1"}
)

func myIP(nodes []string) (string, error) {
	var err error
	for _, iface := range netInterfaces() {
		ip, _, err := ipByInterface(iface.Name)
		if err == nil && stringSlice(nodes).pos(ip) != -1 {
			return ip, nil
		}
	}

	glog.Errorf("error getting local IP: %v", err)
	return "0.0.0.0", err
}

func netInterfaces() []net.Interface {
	r, _ := regexp.Compile(vethRegex)

	validIfaces := []net.Interface{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return validIfaces
	}

	for _, iface := range ifaces {
		if !r.MatchString(iface.Name) && stringSlice(invalidIfaces).pos(iface.Name) == -1 {
			validIfaces = append(validIfaces, iface)
		}
	}

	return validIfaces
}

func interfaceByIP(ip string) string {
	for _, iface := range netInterfaces() {
		ifaceIP, _, err := ipByInterface(iface.Name)
		if err == nil && ip == ifaceIP {
			return iface.Name
		}
	}

	return ""
}

func maskForIP(ip string) int {
	for _, iface := range netInterfaces() {
		ifaceIP, mask, err := ipByInterface(iface.Name)
		if err == nil && ip == ifaceIP {
			return mask
		}
	}

	return 32
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

	return "", 32, errors.New("Found no IPv4 addresses.")
}

type stringSlice []string

func (slice stringSlice) pos(value string) int {
	for p, v := range slice {
		if v == value {
			return p
		}
	}
	return -1
}
