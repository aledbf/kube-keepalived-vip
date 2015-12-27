package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "syscall"
    "testing"
)

// trivial testcase with a single service with a single backend on startup
func TestNewService(t *testing.T) {
    serviceFrontend := config.ServiceFrontend{IPv4:"10.0.1.1", TCP:80}
    serviceBackend := config.ServiceBackend{IPv4:"10.1.0.1", TCP:80}

    services := NewServices()

    services.NewConfig(&config.ConfigService{ConfigSource:"test", ServiceName:"test"})
    services.NewConfig(&config.ConfigServiceFrontend{ConfigSource:"test", ServiceName:"test", Frontend:serviceFrontend})
    services.NewConfig(&config.ConfigServiceBackend{ConfigSource:"test", ServiceName:"test", BackendName:"test1", Backend:serviceBackend})

    // test
    if len(services.services) != 1 {
        t.Errorf("wrong amount of services: %v", services.services)
    }
    service := services.services["test"]
    if service == nil {
        t.Fatalf("missing service: test")
    }
    if service.Name != "test" {
        t.Errorf("Invalid service: name %v", service.Name)
    }
    if service.Frontend == nil || *service.Frontend != serviceFrontend {
        t.Errorf("Invalid service: frontend %v", service.Frontend)
    }
    if len(service.Backends) != 1 {
        t.Errorf("Invalid service backends: %v", service.Backends)
    }
    if service.Backends["test1"] != serviceBackend {
        t.Errorf("Invalid service backend %v: %v", "test1", service.Backends["test1"])
    }
}

func TestServiceSync(t *testing.T) {
    serviceFrontend := config.ServiceFrontend{IPv4:"10.0.1.1", TCP:80}
    serviceBackend := config.ServiceBackend{IPv4:"10.1.0.1", TCP:80}

    services := NewServices()

    services.NewConfig(&config.ConfigService{ConfigSource:"test", ServiceName:"test"})
    services.NewConfig(&config.ConfigServiceFrontend{ConfigSource:"test", ServiceName:"test", Frontend:serviceFrontend})
    services.NewConfig(&config.ConfigServiceBackend{ConfigSource:"test", ServiceName:"test", BackendName:"test1", Backend:serviceBackend})

    // sync
    ipvsDriver, err := services.SyncIPVS(IpvsConfig{FwdMethod: "masq", SchedName: "wlc", mock: true})
    if err != nil {
        t.Fatalf("services.SyncIPVS: %v", err)
    }

    service := services.services["test"]
    if service == nil {
        t.Fatalf("missing service: test")
    }

    // test frontend
    if service.driverFrontend == nil {
        t.Fatalf("missing driverFrontend")
    }
    ipvsType := ipvsType{syscall.AF_INET, syscall.IPPROTO_TCP}
    ipvsService := service.driverFrontend.state[ipvsType]
    if ipvsService == nil {
        t.Fatalf("missing ipvsService %v", ipvsType)
    }
    if ipvsService.String() != "inet+tcp://10.0.1.1:80" {
        t.Errorf("incorrect ipvsService: %v", ipvsService)
    }
    if ipvsService.SchedName != "wlc" {
        t.Errorf("incorrect ipvsService: SchedName=%v", ipvsService.SchedName)
    }

    // test dest
    if service.driverBackends["test1"] == nil {
        t.Fatalf("missing driverBackend: %v", "test1")
    }
    ipvsDest := service.driverBackends["test1"].state[ipvsType]

    if ipvsDest== nil {
        t.Fatalf("did not sync %v", ipvsType)
    }
    if ipvsDest.Addr.String() != "10.1.0.1" {
        t.Errorf("invalid ipvsDest: Addr=%v", ipvsDest.Addr)
    }
    if ipvsDest.Port != 80 {
        t.Errorf("invalid ipvsDest: Port=%v", ipvsDest.Port)
    }
    if ipvsDest.FwdMethod.String() != "masq" {
        t.Errorf("invalid ipvsDest: FwdMethod=%v", ipvsDest.FwdMethod)
    }
    if ipvsDest.Weight != 10 {
        t.Errorf("invalid ipvsDest: Weight=%v", ipvsDest.Weight)
    }

    // test ipvsDriver.dests
    ipvsKey := ipvsKey{"inet+tcp://10.0.1.1:80", "10.1.0.1:80"}

    if len(ipvsDriver.dests) != 1 {
        t.Errorf("incorrect sync dests: %v", ipvsDriver.dests)
    }
    if ipvsDriver.dests[ipvsKey] == nil {
        t.Errorf("missing sync dest: %v", ipvsKey)
    }
    if ipvsDriver.dests[ipvsKey] != ipvsDest {
        t.Errorf("mismatching sync dest: %v", ipvsDriver.dests[ipvsKey])
    }
}

// Test adding a new ConfigServiceFrontend after sync
// https://github.com/qmsk/clusterf/issues/4
func TestServiceAdd(t *testing.T) {
    serviceFrontend := config.ServiceFrontend{IPv4:"10.0.1.1", TCP:80}
    serviceBackend := config.ServiceBackend{IPv4:"10.1.0.1", TCP:80}

    services := NewServices()

    // sync
    ipvsDriver, err := services.SyncIPVS(IpvsConfig{FwdMethod: "masq", SchedName: "wlc", mock: true})
    if err != nil {
        t.Fatalf("services.SyncIPVS: %v", err)
    }

    // add
    services.ConfigEvent(config.Event{Action:config.SetConfig, Config:&config.ConfigServiceFrontend{ConfigSource:"test", ServiceName:"test", Frontend:serviceFrontend}})
    services.ConfigEvent(config.Event{Action:config.SetConfig, Config:&config.ConfigServiceBackend{ConfigSource:"test", ServiceName:"test", BackendName:"test1", Backend:serviceBackend}})

    // test ipvsDriver.dests
    ipvsKey := ipvsKey{"inet+tcp://10.0.1.1:80", "10.1.0.1:80"}

    if len(ipvsDriver.dests) != 1 {
        t.Errorf("incorrect sync dests: %v", ipvsDriver.dests)
    }
    if ipvsDriver.dests[ipvsKey] == nil {
        t.Errorf("missing sync dest: %v", ipvsKey)
    }
}
