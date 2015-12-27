package config

import (
    "encoding/json"
    "strings"
)

func makePath(pathParts ...string) string {
    return strings.Join(pathParts, "/")
}

func makeDirNode(config Config) (Node, error) {
    return Node{Path: config.Path(), IsDir: true}, nil
}

func makeNode(config Config) (Node, error) {
    jsonValue, err := json.Marshal(config.Value())

    return Node{Path: config.Path(), Value: string(jsonValue)}, err
}

func (self ConfigService) Path() string {
    return makePath("services", self.ServiceName)
}
func (self ConfigService) Value() interface{} {
    return nil
}
func (self ConfigService) Source() ConfigSource {
    return self.ConfigSource
}

func (self ConfigServiceFrontend) Path() string {
    return makePath("services", self.ServiceName, "frontend")
}
func (self ConfigServiceFrontend) Value() interface{} {
    return self.Frontend
}
func (self ConfigServiceFrontend) Source() ConfigSource {
    return self.ConfigSource
}

func (self ConfigServiceBackend) Path() string {
    return makePath("services", self.ServiceName, "backends", self.BackendName)
}
func (self ConfigServiceBackend) Value() interface{} {
    return self.Backend
}
func (self ConfigServiceBackend) Source() ConfigSource {
    return self.ConfigSource
}

func (self ConfigRoute) Path() string {
    return makePath("routes", self.RouteName)
}
func (self ConfigRoute) Value() interface{} {
    return self.Route
}
func (self ConfigRoute) Source() ConfigSource {
    return self.ConfigSource
}
