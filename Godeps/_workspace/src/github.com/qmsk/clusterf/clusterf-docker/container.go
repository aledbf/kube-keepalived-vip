package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf/docker"
    "log"
)

type containerState struct {
    id      string
    configs map[string]config.Config
}

func (self containerState) String() string {
    return self.id
}

// Create state for active container and synchronize
func (self *self) newContainer(dockerContainer *docker.Container) *containerState {
    containerState := &containerState{
        id:      dockerContainer.ID,
        configs: make(map[string]config.Config),
    }

    self.syncContainer(containerState, dockerContainer)

    return containerState
}

// Synchronize active container state to config
func (self *self) syncContainer(containerState *containerState, dockerContainer *docker.Container) {
    containerConfigs := configContainer(dockerContainer)

    // TODO: cleanup old configs, if they ever change?
    for _, containerConfig := range containerConfigs {
        if err := self.configEtcd.Publish(containerConfig); err != nil {
            log.Printf("syncContainer %v: publish %v: %v\n", containerState, containerConfig.Path(), err)
        } else {
            log.Printf("syncContainer %v: publish %v: %#v\n", containerState, containerConfig.Path(), containerConfig)

            // succesfully configured; remember for teardown
            containerState.configs[containerConfig.Path()] = containerConfig
        }
    }
}

// Teardown container state if active
func (self *self) teardownContainer(containerState *containerState) {
    for _, containerConfig := range containerState.configs {
        if err := self.configEtcd.Retract(containerConfig); err != nil {
            log.Printf("teardownContainer %v: retract #%v: %v\n", containerState, containerConfig, err)
        } else {
            log.Printf("teardownContainer %v: retract #%v\n", containerState, containerConfig)
        }
    }
}


