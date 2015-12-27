package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/docker"
    "fmt"
    "strings"
    "log"
)

// Translate a docker container to a service config
func configContainer (container *docker.Container) (configs []config.Config) {
    // map ports
    containerPorts := make(map[string]docker.Port)

    for _, port := range container.Ports {
        containerPorts[fmt.Sprintf("%s:%d", port.Proto, port.Port)] = port
    }

    // services
    for _, serviceName := range strings.Fields(container.Labels["net.qmsk.clusterf.service"]) {
        configBackend := config.ConfigServiceBackend{
            ServiceName: serviceName,
            BackendName: container.ID,
        }

        if container.IPv4 != nil {
            configBackend.Backend.IPv4 = container.IPv4.String()
        }

        // find potential ports for service by label
        portLabels := []struct{
            proto string
            label string
        }{
            {"tcp", "net.qmsk.clusterf.backend.tcp"},
            {"udp", "net.qmsk.clusterf.backend.udp"},
            {"tcp", fmt.Sprintf("net.qmsk.clusterf.backend:%s.tcp", serviceName)},
            {"udp", fmt.Sprintf("net.qmsk.clusterf.backend:%s.udp", serviceName)},
        }

        for _, portLabel := range portLabels {
            // lookup exposed docker.Port
            portName, labelFound := container.Labels[portLabel.label]
            if !labelFound {
                continue
            }

            port, portFound := containerPorts[fmt.Sprintf("%s:%s", portLabel.proto, portName)]
            if !portFound {
                log.Printf("configContainer %v: service %v port %v is not exposed\n", container, serviceName, portName)
                continue
            }

            // configure
            switch port.Proto {
            case "tcp":
                configBackend.Backend.TCP = port.Port
            case "udp":
                configBackend.Backend.UDP = port.Port
            }
        }

        if configBackend.Backend.TCP != 0 || configBackend.Backend.UDP != 0 {
            configs = append(configs, configBackend)
        }
    }

    return
}
