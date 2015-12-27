package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "fmt"
    "github.com/qmsk/clusterf/ipvs"
    "log"
    "net"
    "syscall"
)

const IPVS_WEIGHT uint32 = 10

type ipvsBackend struct {
    driver      *IPVSDriver
    frontend    *ipvsFrontend
    state       map[ipvsType]*ipvs.Dest
    weight      uint32
}

func makeBackend(frontend *ipvsFrontend) *ipvsBackend {
    return &ipvsBackend{
        driver:     frontend.driver,
        frontend:   frontend,
        state:      make(map[ipvsType]*ipvs.Dest),
    }
}

func (self *ipvsBackend) buildDest (ipvsService *ipvs.Service, backend config.ServiceBackend) (*ipvs.Dest, error) {
    ipvsDest := &ipvs.Dest{
        FwdMethod:  self.driver.fwdMethod,
    }

    switch ipvsService.Af {
    case syscall.AF_INET:
        if backend.IPv4 == "" {
            return nil, nil
        } else if ip := net.ParseIP(backend.IPv4); ip == nil {
            return nil, fmt.Errorf("Invalid IPv4: %v", backend.IPv4)
        } else if ip4 := ip.To4(); ip4 == nil {
            return nil, fmt.Errorf("Invalid IPv4: %v", ip)
        } else {
            ipvsDest.Addr = ip4
        }
    case syscall.AF_INET6:
        if backend.IPv6 == "" {
            return nil, nil
        } else if ip := net.ParseIP(backend.IPv6); ip == nil {
            return nil, fmt.Errorf("Invalid IPv6: %v", backend.IPv6)
        } else if ip16 := ip.To16(); ip16 == nil {
            return nil, fmt.Errorf("Invalid IPv6: %v", ip)
        } else {
            ipvsDest.Addr = ip16
        }
    default:
        panic("invalid af")
    }

    switch ipvsService.Protocol {
    case syscall.IPPROTO_TCP:
        if backend.TCP == 0 {
            return nil, nil
        } else {
            ipvsDest.Port = backend.TCP
        }
    case syscall.IPPROTO_UDP:
        if backend.UDP == 0 {
            return nil, nil
        } else {
            ipvsDest.Port = backend.UDP
        }
    default:
        panic("invalid proto")
    }

    if backend.Weight == 0 {

    } else {
        ipvsDest.Weight = uint32(backend.Weight)
    }

    return self.applyRoute(ipvsService, ipvsDest)
}

func (self *ipvsBackend) applyRoute (ipvsService *ipvs.Service, ipvsDest *ipvs.Dest) (*ipvs.Dest, error) {
    route := self.driver.routes.Lookup(ipvsDest.Addr)
    if route == nil {
        return ipvsDest, nil
    }

    log.Printf("cluster:ipvsBackend.applyRoute %v: %v\n", ipvsDest, route)

    if route.ipvs_filter {
        // ignore
        return nil, nil
    }

    if route.ipvs_fwdMethod != nil {
        ipvsDest.FwdMethod = *route.ipvs_fwdMethod
    }

    switch ipvsService.Af {
    case syscall.AF_INET:
        if route.Gateway4 != nil {
            // chaining
            ipvsDest.Addr = route.Gateway4
            ipvsDest.Port = ipvsService.Port
        }
    }

    return ipvsDest, nil
}

func (self *ipvsBackend) updateWeight(weight uint) {
    if weight == 0 {
        self.weight = IPVS_WEIGHT
    } else {
        self.weight = uint32(weight) // XXX: check
    }
}

// create any instances of this backend, assuming there is no active state
func (self *ipvsBackend) add(backend config.ServiceBackend) error {
    self.updateWeight(backend.Weight)

    for _, ipvsType := range ipvsTypes {
        if ipvsService := self.frontend.state[ipvsType]; ipvsService != nil {
            ipvsDest, err := self.buildDest(ipvsService, backend)

            if err != nil {
                // XXX: continue
                return err
            }
            if ipvsDest == nil {
                continue
            }

            if upDest, err := self.driver.upDest(ipvsService, ipvsDest, self.weight); err != nil {
                return err
            } else {
                self.state[ipvsType] = upDest
            }
        }
    }

    return nil
}

// update any instances of this backend
// - removes any active instances that are no longer configured
// - replaces any active instances that have changed
// - adds new active isntances that are now configured
//
// TODO: sets any active instances that have changed parameters
func (self *ipvsBackend) set(backend config.ServiceBackend) error {
    getWeight := self.weight
    self.updateWeight(backend.Weight)
    setWeight := self.weight

    for _, ipvsType := range ipvsTypes {
        if ipvsService := self.frontend.state[ipvsType]; ipvsService != nil {
            var setDest, getDest *ipvs.Dest
            var match bool

            getDest = self.state[ipvsType]

            if ipvsDest, err := self.buildDest(ipvsService, backend); err != nil {
                return err
            } else if ipvsDest != nil {
                setDest = ipvsDest
            }

            // compare for matching id, but changed value
            if setDest == nil || getDest == nil {
                match = false
            } else if setDest.String() == getDest.String() {
                match = true
            } else {
                match = false
            }

            if setDest == nil {
                // configured as inactive
            } else if match {
                log.Printf("clusterf:ipvsBackend.set: set %v %v +%d-%d\n", ipvsService, setDest, setWeight, getWeight)

                // XXX: fwdMethod?
                // update existing ipvs.Dest in-place
                if err := self.driver.adjustDest(ipvsService, getDest, int(setWeight) - int(getWeight)); err != nil  {
                    return err
                }

                setDest = getDest

            } else {
                log.Printf("clusterf:ipvsBackend.set: new %v %v\n", ipvsService, setDest)

                // replace active
                if upDest, err := self.driver.upDest(ipvsService, setDest, setWeight); err != nil {
                    return err
                } else {
                    setDest = upDest
                }
            }

            // may be nil, if the new backend did not have this ipvsType
            self.state[ipvsType] = setDest

            if getDest == nil {
                // not active

            } else if match {
                // remains active

            } else {
                log.Printf("clusterf:ipvsBackend.set: del %v %v\n", ipvsService, getDest)

                // replace active
                if err := self.driver.downDest(ipvsService, getDest, getWeight); err != nil {
                    // XXX: inconsistent, we now have two dest's
                    return err
                }
            }
        }
    }

    return nil
}

// remove any active instances of this backend, clearing the active state
func (self *ipvsBackend) del() error {
    for _, ipvsType := range ipvsTypes {
        if ipvsService := self.frontend.state[ipvsType]; ipvsService != nil {
            if ipvsDest := self.state[ipvsType]; ipvsDest != nil {
                if err := self.driver.downDest(ipvsService, ipvsDest, self.weight); err != nil {
                    return err
                }

                self.state[ipvsType] = nil
            }
        }
    }

    return nil
}
