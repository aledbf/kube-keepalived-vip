package clusterf
/*
 * Internal services states, maintained from config changes.
 */

import (
    "github.com/qmsk/clusterf/config"
    "fmt"
    "log"
)

type Services struct {
    services    map[string]*Service
    routes      Routes

    driver      *IPVSDriver
}

func NewServices() *Services {
    return &Services{
        services:   make(map[string]*Service),
        routes:     makeRoutes(),
    }
}

// Return Service for named service, possibly creating a new (empty) Service.
func (self *Services) get(name string) *Service {
    service, serviceExists := self.services[name]

    if !serviceExists {
        service = newService(name)
        self.services[name] = service

        // initial sync
        if self.driver != nil {
            service.sync(self.driver)
        }
    }

    return service
}

// Return all currently valid Services
func (self *Services) Services() []*Service {
    services := make([]*Service, 0, len(self.services))

    for _, service := range self.services {
        if service.Frontend == nil {
            continue
        }

        services = append(services, service)
    }

    return services
}

/* Configuration actions */

// Configuration action on a service itself
// handle service-delete actions
// new service creation is implicitly handled when calling this
func (self *Services) configService(service *Service, action config.Action, serviceConfig *config.ConfigService) {
    log.Printf("clusterf:Service %s: %s %+v\n", service.Name, action, serviceConfig)

    switch action {
    case config.DelConfig:
        delete(self.services, service.Name)

        service.delFrontend()
    }
}

func (self *Services) configRoute(route *Route, action config.Action, routeConfig *config.ConfigRoute) {
    log.Printf("clusterf:Route %s: %s %+v\n", route.Name, action, routeConfig)

    switch action {
    case config.NewConfig, config.SetConfig:
        if err := route.config(action, routeConfig.Route); err != nil {
            log.Printf("clusterf:Route %s: %s\n", route.Name, err)
        } else {
            log.Printf("clusterf:Route %s: %+v\n", route.Name, route)
        }

    case config.DelConfig:
        self.routes.del(route.Name)
    }

    // TODO: update services?
}

func (self *Services) config(action config.Action, baseConfig config.Config) {
    log.Printf("clusterf: config %s %#v\n", action, baseConfig)

    switch applyConfig:= baseConfig.(type) {
    case *config.ConfigService:
        serviceConfig := baseConfig.(*config.ConfigService)

        if serviceConfig.ServiceName == "" {
            // all services
            for _, service := range self.services {
                self.configService(service, action, serviceConfig)
            }
        } else {
            service := self.get(serviceConfig.ServiceName)

            self.configService(service, action, serviceConfig)
        }

    case *config.ConfigServiceFrontend:
        frontendConfig := baseConfig.(*config.ConfigServiceFrontend)

        service := self.get(frontendConfig.ServiceName)

        service.configFrontend(action, frontendConfig)

    case *config.ConfigServiceBackend:
        backendConfig := baseConfig.(*config.ConfigServiceBackend)

        service := self.get(backendConfig.ServiceName)

        if backendConfig.BackendName == "" {
            // all service backends
            for backendName, _ := range service.Backends {
                service.configBackend(backendName, action, backendConfig)
            }
        } else {
            service.configBackend(backendConfig.BackendName, action, backendConfig)
        }

    case *config.ConfigRoute:
        if applyConfig.RouteName == "" {
            // all routes
            for _, route := range self.routes {
                self.configRoute(route, action, applyConfig)
            }
        } else {
            route := self.routes.get(applyConfig.RouteName)

            self.configRoute(route, action, applyConfig)
        }

    default:
        panic(fmt.Errorf("Unknown config type: %#v", baseConfig))
    }
}

// Initialize config before driver sync
func (self *Services) NewConfig(baseConfig config.Config) {
    if self.driver != nil {
        panic("NewConfig after driver sync")
    }

    self.config(config.NewConfig, baseConfig)
}

// Sync initial configuration loaded via NewConfig() to IPVS
//
// Begins by flushing the IPVS state
func (self *Services) SyncIPVS(ipvsConfig IpvsConfig) (*IPVSDriver, error) {
    if ipvsDriver, err := ipvsConfig.setup(self.routes); err != nil {
        return nil, err
    } else {
        self.driver = ipvsDriver
    }

    // begin sync
    if err := self.driver.sync(); err != nil {
        return nil, err
    }

    for _, service := range self.services {
        service.sync(self.driver)
    }

    return self.driver, nil
}

// Apply changes to the current configuration, updating the running driver
func (self *Services) ConfigEvent(event config.Event) {
    if self.driver == nil {
        panic("ConfigEvent before driver sync")
    }

    self.config(event.Action, event.Config)
}
