package clusterf
/*
 * Internal service state, maintained from config changes.
 */

import (
    "github.com/qmsk/clusterf/config"
    "log"
)

type Service struct {
    Name        string

    Frontend    *config.ServiceFrontend
    Backends    map[string]config.ServiceBackend

    driverFrontend  *ipvsFrontend
    driverBackends  map[string]*ipvsBackend
}

func newService(name string) *Service {
    return &Service{
        Name:           name,
        Backends:       make(map[string]config.ServiceBackend),

        driverBackends: make(map[string]*ipvsBackend),
    }
}

func (self *Service) driverError(err error) {
    log.Printf("cluster:Service %s: Error: %s\n", self.Name, err)
}

/* Configuration actions */
func (self *Service) configFrontend(action config.Action, frontendConfig *config.ConfigServiceFrontend) {
    frontend := frontendConfig.Frontend

    log.Printf("clusterf:Service %s: Frontend: %s %+v <- %+v\n", self.Name, action, frontend, self.Frontend)

    switch action {
    case config.NewConfig:
        self.Frontend = &frontend

    case config.SetConfig:
        if self.Frontend == nil {
            self.newFrontend(frontend)
        } else if *self.Frontend != frontend {
            self.setFrontend(frontend)
        }

        self.Frontend = &frontend

    case config.DelConfig:
        self.delFrontend()

        self.Frontend = nil
    }
}

func (self *Service) configBackend(backendName string, action config.Action, backendConfig *config.ConfigServiceBackend) {
    log.Printf("clusterf:Service %s: Backend %s: %s %+v <- %+v\n", self.Name, backendName, action, backendConfig.Backend, self.Backends[backendName])

    switch action {
    case config.NewConfig:
        self.Backends[backendName] = backendConfig.Backend

    case config.SetConfig:
        if self.Backends[backendName] == backendConfig.Backend {
            return
        }

        if self.Frontend != nil {
            self.setBackend(backendName, backendConfig.Backend)
        }

        self.Backends[backendName] = backendConfig.Backend

    case config.DelConfig:
        if self.Frontend != nil {
            self.delBackend(backendName)
        }

        delete(self.Backends, backendName)
    }
}

// Synchronize state to IPVS
func (self *Service) sync(driver *IPVSDriver) {
    self.driverFrontend = driver.newFrontend()

    if self.Frontend != nil {
        // also adds backends
        self.newFrontend(*self.Frontend)
    }
}

/* Frontend actions */
func (self *Service) newFrontend(frontend config.ServiceFrontend) {
    log.Printf("clusterf:Service %s: new Frontend: %+v\n", self.Name, frontend)

    if err := self.driverFrontend.add(frontend); err != nil {
        self.driverError(err)
    }

    for backendName, backend := range self.Backends {
        self.newBackend(backendName, backend)
    }
}

func (self *Service) setFrontend(frontend config.ServiceFrontend) {
    log.Printf("clusterf:Service %s: set Frontend: %+v\n", self.Name, frontend)

    if self.Frontend != nil {
        // TODO: smoother setup-before-teardown transition..?
        self.delFrontend()
    }

    self.newFrontend(frontend)
}

func (self *Service) delFrontend() {
    log.Printf("clusterf:Service %s: del Frontend: %+v\n", self.Name, self.Frontend)

    // del'ing the frontend will also remove all backend state
    if err := self.driverFrontend.del(); err != nil {
        self.driverError(err)
    }

    // clear backend state
    for backendName, _ := range self.driverBackends {
        delete(self.driverBackends, backendName)
    }
}

/* Backend actions */
func (self *Service) newBackend(backendName string, backend config.ServiceBackend) {
    log.Printf("clusterf:Service %s: new Backend %s: %+v\n", self.Name, backendName, backend)

    self.driverBackends[backendName] = self.driverFrontend.newBackend()

    if err := self.driverBackends[backendName].add(backend); err != nil {
        self.driverError(err)
    }
}

func (self *Service) setBackend(backendName string, backend config.ServiceBackend) {
    log.Printf("clusterf:Service %s: set Backend %s: %+v\n", self.Name, backendName, backend)

    if driverBackend := self.driverBackends[backendName]; driverBackend == nil {
        self.newBackend(backendName, backend)
    } else if err := driverBackend.set(backend); err != nil {
        self.driverError(err)
    }
}

func (self *Service) delBackend(backendName string) {
    log.Printf("clusterf:Service %s: del Backend %s: %+v\n", self.Name, backendName, self.Backends[backendName])

    if err := self.driverBackends[backendName].del(); err != nil {
        self.driverError(err)
    }

    delete(self.driverBackends, backendName)
}
