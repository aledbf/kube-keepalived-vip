package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/docker"
    "flag"
    "log"
    "os"
)

var (
    dockerConfig docker.DockerConfig
    etcdConfig  config.EtcdConfig
)

func init() {
    flag.StringVar(&dockerConfig.Endpoint, "docker-endpoint", "",
        "Docker client endpoint for dockerd")

    flag.StringVar(&etcdConfig.Machines, "etcd-machines", "http://127.0.0.1:2379",
        "Client endpoint for etcd")
    flag.StringVar(&etcdConfig.Prefix, "etcd-prefix", "/clusterf",
        "Etcd tree prefix")
}

type self struct {
    configEtcd  *config.Etcd
    docker      *docker.Docker

    // registered state
    containers  map[string]*containerState
}

// Update container state from docker API
func (self *self) containerEvent(containerEvent docker.ContainerEvent) {
    containerState := self.containers[containerEvent.ID]

    if containerEvent.Running {
        if containerEvent.State == nil {
            log.Printf("containerEvent %v: unknown\n", containerEvent)

        } else if containerState == nil {
            log.Printf("containerEvent %v: new\n", containerEvent)

            self.containers[containerEvent.ID] = self.newContainer(containerEvent.State)

        } else {
            log.Printf("containerEvent %v sync\n", containerEvent)

            self.syncContainer(containerState, containerEvent.State)
        }
    } else {
        if containerState == nil {
            log.Printf("containerEvent %v: skip\n", containerEvent)
        } else {
            log.Printf("containerEvent %v: teardown\n", containerEvent)

            self.teardownContainer(containerState)

            delete(self.containers, containerEvent.ID)
        }
    }
}

func main() {
    self := self{
        containers:    make(map[string]*containerState),
    }

    flag.Parse()

    if len(flag.Args()) > 0 {
        flag.Usage()
        os.Exit(1)
    }

    if configEtcd, err := etcdConfig.Open(); err != nil {
        log.Fatalf("config:etcd.Open: %v\n", err)
    } else {
        log.Printf("config:etcd.Open: %v\n", configEtcd)

        self.configEtcd = configEtcd
    }

    if docker, err := dockerConfig.Open(); err != nil {
        log.Fatalf("docker:Docker.Open: %v\n", err)
    } else {
        log.Printf("docker:Docker.Open: %v\n", docker)

        self.docker = docker
    }

    // sync
    if containerEvents, err := self.docker.Sync(); err != nil {
        log.Fatalf("docker:Docker.Sync: %v\n", err)
    } else {
        log.Printf("docker:Docker.Sync...\n")

        for containerEvent := range containerEvents {
            self.containerEvent(containerEvent)
        }
    }
}
