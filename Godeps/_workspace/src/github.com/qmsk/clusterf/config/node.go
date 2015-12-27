package config

import (
    "fmt"
    "encoding/json"
    "strings"
)

type Node struct {
    // clusterf-relative path, so with any prefix stripped
    Path    string
    IsDir   bool

    // json-encoded
    Value   string

    Source  ConfigSource
}

func (self *Node) loadServiceFrontend() (frontend ServiceFrontend, err error) {
    err = json.Unmarshal([]byte(self.Value), &frontend)

    return
}

func (self *Node) loadServiceBackend() (backend ServiceBackend, err error) {
    err = json.Unmarshal([]byte(self.Value), &backend)

    return
}

func (self *Node) loadRoute() (route Route, err error) {
    err = json.Unmarshal([]byte(self.Value), &route)

    return
}

// map config node path and value to Config
func syncConfig(node Node) (Config, error) {
    nodePath := strings.Split(node.Path, "/")

    if len(node.Path) == 0 {
        // Split("", "/") would give [""]
        nodePath = nil
    }

    // match config tree path
    if len(nodePath) == 0 && node.IsDir {
        // XXX: just ignore? Undefined if it makes sense to do anything here
        return nil, nil

    } else if len(nodePath) == 1 && nodePath[0] == "services" && node.IsDir {
        // recursive on all services
        return &ConfigService{ConfigSource: node.Source}, nil

    } else if len(nodePath) >= 2 && nodePath[0] == "services" {
        serviceName := nodePath[1]

        if len(nodePath) == 2 && node.IsDir {
            return &ConfigService{ServiceName: serviceName, ConfigSource: node.Source}, nil

        } else if len(nodePath) == 3 && nodePath[2] == "frontend" && !node.IsDir {
            if node.Value == "" {
                // deleted node has empty value
                return &ConfigServiceFrontend{ServiceName: serviceName, ConfigSource: node.Source}, nil
            } else if frontend, err := node.loadServiceFrontend(); err != nil {
                return nil, fmt.Errorf("service %s frontend: %s", serviceName, err)
            } else {
                return &ConfigServiceFrontend{ServiceName: serviceName, Frontend: frontend, ConfigSource: node.Source}, nil
            }

        } else if len(nodePath) == 3 && nodePath[2] == "backends" && node.IsDir {
            // recursive on all backends
            return &ConfigServiceBackend{ServiceName: serviceName, ConfigSource: node.Source}, nil

        } else if len(nodePath) >= 4 && nodePath[2] == "backends" {
            backendName := nodePath[3]

            if len(nodePath) == 4 && !node.IsDir {
                if node.Value == "" {
                    // deleted node has empty value
                    return &ConfigServiceBackend{ServiceName: serviceName, BackendName: backendName, ConfigSource: node.Source}, nil
                } else if backend, err := node.loadServiceBackend(); err != nil {
                    return nil, fmt.Errorf("service %s backend %s: %s", serviceName, backendName, err)
                } else {
                    return &ConfigServiceBackend{ServiceName: serviceName, BackendName: backendName, Backend: backend, ConfigSource: node.Source}, nil
                }

            } else {
                return nil, fmt.Errorf("Ignore unknown service %s backends node", serviceName)
            }

        } else {
            return nil, fmt.Errorf("Ignore unknown service %s node", serviceName)
        }

    } else if len(nodePath) == 1 && nodePath[0] == "routes" && node.IsDir {
        // recursive on all routes
        return &ConfigRoute{ }, nil

    } else if len(nodePath) >= 2 && nodePath[0] == "routes" {
        routeName := nodePath[1]

        if len(nodePath) == 2 && !node.IsDir {
            if node.Value == "" {
                // deleted node has empty value
                return &ConfigRoute{RouteName: routeName, ConfigSource: node.Source}, nil
            } else if route, err := node.loadRoute(); err != nil {
                return nil, fmt.Errorf("route %s: %s", routeName, err)
            } else {
                return &ConfigRoute{RouteName: routeName, Route: route, ConfigSource: node.Source}, nil
            }
        } else {
            return nil, fmt.Errorf("Ignore unknown route node")
        }
    } else {
        return nil, fmt.Errorf("Ignore unknown node")
    }

    return nil, nil
}

func syncEvent(action Action, node Node) (*Event, error) {
    // match
    if config, err := syncConfig(node); err != nil {
        return nil, err
    } else if config == nil {
        return nil, nil
    } else {
        return &Event{Action: action, Config: config}, nil
    }
}
