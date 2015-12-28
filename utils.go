package main

import (
	"errors"
	"fmt"
	"net"
)

func myIP(nodes []string) (string, error) {
	maxIface := 5
	var err error
	for i := 0; i < maxIface; i++ {
		var ip string
		ip, err = IPByInterface(fmt.Sprintf("eth%d", i))
		if err == nil && stringSlice(nodes).pos(ip) != -1 {
			return ip, nil
		}
	}

	return "0.0.0.0", err
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
