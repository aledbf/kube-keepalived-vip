package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "fmt"
    "testing"
)

// Test multiple NewConfig for Routes from multiple config sources
// https://github.com/qmsk/clusterf/issues/7
func TestNewConfigRoute(t *testing.T) {
    services := NewServices()

    // initial configs from one source
    services.NewConfig(&config.ConfigRoute{RouteName:""})
    services.NewConfig(&config.ConfigRoute{RouteName:"test", Route:config.Route{Prefix4:"10.0.0.0/24", IpvsMethod:"droute"}})

    route := services.routes["test"]

    if route == nil {
        t.Errorf("test route not configured")
        return
    }
    if fmt.Sprintf("%v", route.Prefix4) != "10.0.0.0/24" || route.ipvs_fwdMethod.String() != "droute" {
        t.Errorf("test route mis-configured: %#v", route)
        return
    }

    // second round of configs from a different sources
    services.NewConfig(&config.ConfigRoute{RouteName:""})
    route2 := services.routes["test"]

    if route2 == nil {
        t.Errorf("test route disappeared")
        return
    }
    if fmt.Sprintf("%v", route2.Prefix4) != "10.0.0.0/24" || route2.ipvs_fwdMethod.String() != "droute" {
        t.Errorf("test route mis-configured: %#v", route2)
        return
    }
}

// test Services.ConfigEvent(config.DelConfig, config.ConfigRoute{RouteName:""})
func TestDelConfigRoutes(t *testing.T) {
    services := NewServices()

    // initial configs from one source
    services.NewConfig(&config.ConfigRoute{ConfigSource: "test1", RouteName:""})
    services.NewConfig(&config.ConfigRoute{ConfigSource: "test1", RouteName:"test1", Route:config.Route{Prefix4:"10.0.1.0/24", IpvsMethod:"droute"}})

    // initial configs from second source
    services.NewConfig(&config.ConfigRoute{ConfigSource: "test2", RouteName:""})
    services.NewConfig(&config.ConfigRoute{ConfigSource: "test2", RouteName:"test2", Route:config.Route{Prefix4:"10.0.2.0/24", IpvsMethod:"droute"}})

    // sync
    if _, err := services.SyncIPVS(IpvsConfig{mock: true}); err != nil {
        t.Fatalf("services.SyncIPVS: %v", err)
    }

    // delete second source
    services.ConfigEvent(config.Event{Action: config.DelConfig, Config: &config.ConfigRoute{RouteName:"", ConfigSource:"test2"}})

    // test first source's route
    route1 := services.routes["test1"]
    route2 := services.routes["test2"]

    if route1 == nil {
        t.Errorf("test1 Route got deleted after recursive test2 DelConfig")
    }
    if route2 != nil {
        t.Errorf("test2 route remains after recursive test2 DelConfig")
    }
}
