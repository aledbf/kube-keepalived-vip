package main

import (
	"errors"
	"net"
	"regexp"
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
		ip, err := IPByInterface(iface.Name)
		if err == nil && stringSlice(nodes).pos(ip) != -1 {
			return ip, nil
		}
	}

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

func IPByInterface(name string) (string, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	var ip string
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
			}
		}
	}
	if len(ip) == 0 {
		return ip, errors.New("Found no IPv4 addresses.")
	}
	return ip, nil
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
