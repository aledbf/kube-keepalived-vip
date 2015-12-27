package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "fmt"
    "github.com/qmsk/clusterf/ipvs"
    "log"
    "net"
    "syscall"
)

type ipvsFrontend struct {
    driver      *IPVSDriver
    state       map[ipvsType]*ipvs.Service
}

func makeFrontend(driver *IPVSDriver) *ipvsFrontend {
    return &ipvsFrontend{
        driver: driver,
        state:  make(map[ipvsType]*ipvs.Service),
    }
}

func (self *ipvsFrontend) newBackend() *ipvsBackend {
    return makeBackend(self)
}

// setup a valid ipvs.Service for the given ServiceFrontend and ipvsType
// returns is-valid, error
func (self *ipvsFrontend) buildService (ipvsType ipvsType, frontend config.ServiceFrontend) (*ipvs.Service, error) {
    ipvsService := &ipvs.Service{
        Af:         ipvsType.Af,
        Protocol:   ipvsType.Protocol,

        SchedName:  self.driver.schedName,
        Timeout:    0,
        Flags:      ipvs.Flags{Flags: 0, Mask: 0xffffffff},
        Netmask:    0xffffffff,
    }

    switch ipvsType.Af {
    case syscall.AF_INET:
        if frontend.IPv4 == "" {
            return nil, nil
        } else if ip := net.ParseIP(frontend.IPv4); ip == nil {
            return nil, fmt.Errorf("Invalid IPv4: %v", frontend.IPv4)
        } else if ip4 := ip.To4(); ip4 == nil {
            return nil, fmt.Errorf("Invalid IPv4: %v", ip)
        } else {
            ipvsService.Addr = ip4
        }
    case syscall.AF_INET6:
        if frontend.IPv6 == "" {
            return nil, nil
        } else if ip := net.ParseIP(frontend.IPv6); ip == nil {
            return nil, fmt.Errorf("Invalid IPv6: %v", frontend.IPv6)
        } else if ip16 := ip.To16(); ip16 == nil {
            return nil, fmt.Errorf("Invalid IPv6: %v", ip)
        } else {
            ipvsService.Addr = ip16
        }
    }

    switch ipvsType.Protocol {
    case syscall.IPPROTO_TCP:
        if frontend.TCP == 0 {
            return nil, nil
        } else {
            ipvsService.Port = frontend.TCP
        }
    case syscall.IPPROTO_UDP:
        if frontend.UDP == 0 {
            return nil, nil
        } else {
            ipvsService.Port = frontend.UDP
        }
    default:
        panic("invalid proto")
    }

    return ipvsService, nil
}

func (self *ipvsFrontend) add(frontend config.ServiceFrontend) error {
    for _, ipvsType := range ipvsTypes {
        if ipvsService, err := self.buildService(ipvsType, frontend); err != nil {
            return err
        } else if ipvsService != nil {
            log.Printf("clusterf:ipvsFrontend.add: new %v\n", ipvsService)

            if err := self.driver.upService(ipvsService); err != nil  {
                return err
            } else {
                self.state[ipvsType] = ipvsService
            }
        }
    }

    return nil
}

func (self *ipvsFrontend) del() error {
    for _, ipvsType := range ipvsTypes {
        if ipvsService := self.state[ipvsType]; ipvsService != nil {
            log.Printf("clusterf:ipvsFrontend.del: del %v\n", ipvsService)

            if err := self.driver.downService(ipvsService); err != nil  {
                return err
            } else {
                self.state[ipvsType] = nil
            }
        }
    }

    return nil
}
