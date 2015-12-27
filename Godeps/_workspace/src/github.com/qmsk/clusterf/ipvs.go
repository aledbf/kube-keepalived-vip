package clusterf

import (
    "fmt"
    "github.com/qmsk/clusterf/ipvs"
    "log"
    "syscall"
)

const IPVS_FWD_METHOD = ipvs.IP_VS_CONN_F_MASQ
const IPVS_SCHED_NAME = "wlc"

type ipvsType struct {
    Af          ipvs.Af
    Protocol    ipvs.Protocol
}

var ipvsTypes = []ipvsType {
    { syscall.AF_INET,      syscall.IPPROTO_TCP },
    { syscall.AF_INET6,     syscall.IPPROTO_TCP },
    { syscall.AF_INET,      syscall.IPPROTO_UDP },
    { syscall.AF_INET6,     syscall.IPPROTO_UDP },
}

type ipvsKey struct {
    Service     string
    Dest        string
}

type IpvsConfig struct {
    Debug       bool
    FwdMethod   string
    SchedName   string
    mock        bool        // used for testing; do not actually setup the ipvsClient
}

type IPVSDriver struct {
    ipvsClient *ipvs.Client

    // global state
    routes      Routes

    // deduplicate overlapping destinations
    dests       map[ipvsKey]*ipvs.Dest

    // global defaults
    fwdMethod   ipvs.FwdMethod
    schedName   string
}

func (self IpvsConfig) setup(routes Routes) (*IPVSDriver, error) {
    driver := &IPVSDriver{
        routes: routes,
        dests:  make(map[ipvsKey]*ipvs.Dest),
    }

    if self.FwdMethod == "" {
        driver.fwdMethod = IPVS_FWD_METHOD
    } else if fwdMethod, err := ipvs.ParseFwdMethod(self.FwdMethod); err != nil {
        return nil, err
    } else {
        driver.fwdMethod = fwdMethod
    }

    if self.SchedName == "" {
        driver.schedName = IPVS_SCHED_NAME
    } else {
        driver.schedName = self.SchedName
    }

    // IPVS
    if self.mock {

    } else if ipvsClient, err := ipvs.Open(); err != nil {
        return nil, err
    } else {
        log.Printf("ipvs.Open: %+v\n", ipvsClient)

        driver.ipvsClient = ipvsClient
    }

    if driver.ipvsClient != nil && self.Debug {
        driver.ipvsClient.SetDebug()
    }

    if driver.ipvsClient == nil {
        // mock'd
    } else if info, err := driver.ipvsClient.GetInfo(); err != nil {
        return nil, err
    } else {
        log.Printf("ipvs.GetInfo: version=%s, conn_tab_size=%d\n", info.Version, info.ConnTabSize)
    }

    return driver, nil
}

// Begin initial config sync by flushing the system state
func (self *IPVSDriver) sync() error {
    if self.ipvsClient == nil {

    } else if err := self.ipvsClient.Flush(); err != nil {
        return err
    } else {
        log.Printf("ipvs.Flush")
    }

    return nil
}

func (self *IPVSDriver) newFrontend() *ipvsFrontend {
    return makeFrontend(self)
}

func (self *IPVSDriver) upService(ipvsService *ipvs.Service) error {
    if self.ipvsClient == nil {

    } else if err := self.ipvsClient.NewService(*ipvsService); err != nil  {
        return err
    }

    return nil
}

// bring up a service-dest with given weight, mergeing if necessary
func (self *IPVSDriver) upDest(ipvsService *ipvs.Service, ipvsDest *ipvs.Dest, weight uint32) (*ipvs.Dest, error) {
    ipvsKey := ipvsKey{ipvsService.String(), ipvsDest.String()}

    if mergeDest, mergeExists := self.dests[ipvsKey]; !mergeExists {
        ipvsDest.Weight = weight

        log.Printf("clusterf:ipvs upDest: new %v %v\n", ipvsService, ipvsDest)

        if self.ipvsClient == nil {
        } else if err := self.ipvsClient.NewDest(*ipvsService, *ipvsDest); err != nil {
            return ipvsDest, err
        }

        self.dests[ipvsKey] = ipvsDest

        return ipvsDest, nil

    } else {
        log.Printf("clusterf:ipvs upDest: merge %v %v +%d\n", ipvsService, mergeDest, weight)

        mergeDest.Weight += weight

        if self.ipvsClient == nil {

        } else if err := self.ipvsClient.SetDest(*ipvsService, *mergeDest); err != nil {
            return mergeDest, err
        }

        return mergeDest, nil
    }
}

// update an existing dest with a new weight
func (self *IPVSDriver) adjustDest(ipvsService *ipvs.Service, ipvsDest *ipvs.Dest, weightDelta int) error {
    ipvsKey := ipvsKey{ipvsService.String(), ipvsDest.String()}

    if mergeDest := self.dests[ipvsKey]; mergeDest != ipvsDest {
        panic(fmt.Errorf("invalid dest %#v should be %#v", ipvsDest, mergeDest))
    }

    ipvsDest.Weight = uint32(int(ipvsDest.Weight) + weightDelta)

    // reconfigure active in-place
    if self.ipvsClient == nil {

    } else if err := self.ipvsClient.SetDest(*ipvsService, *ipvsDest); err != nil  {
        return err
    }

    return nil
}

// bring down a service-dest with given weight, merging if necessary
func (self *IPVSDriver) downDest(ipvsService *ipvs.Service, ipvsDest *ipvs.Dest, weight uint32) error {
    ipvsKey := ipvsKey{ipvsService.String(), ipvsDest.String()}

    if mergeDest := self.dests[ipvsKey]; mergeDest != ipvsDest {
        panic(fmt.Errorf("invalid dest %#v should be %#v", ipvsDest, mergeDest))
    }

    if ipvsDest.Weight > weight {
        log.Printf("clusterf:ipvs downDest: merge %v %v -%d\n", ipvsService, ipvsDest, weight)

        ipvsDest.Weight -= weight

        if self.ipvsClient == nil {

        } else if err := self.ipvsClient.SetDest(*ipvsService, *ipvsDest); err != nil {
            return err
        }

    } else if ipvsDest.Weight < weight {
        panic(fmt.Errorf("invalid weight %d for dest %#v", weight, ipvsDest))

    } else {
        log.Printf("clusterf:ipvs downdest: del %v %v\n", ipvsService, ipvsDest)

        if self.ipvsClient == nil {

        } else if err := self.ipvsClient.DelDest(*ipvsService, *ipvsDest); err != nil  {
            return err
        }

        delete(self.dests, ipvsKey)
    }

    return nil
}

func (self *IPVSDriver) downService(ipvsService *ipvs.Service) error {
    if self.ipvsClient == nil {

    } else if err := self.ipvsClient.DelService(*ipvsService); err != nil {
        return err
    }

    // flush any dests, since the kernel will also clear them out
    for ipvsKey, _ := range self.dests {
        if ipvsService.String() == ipvsKey.Service {
            delete(self.dests, ipvsKey)
        }
    }

    return nil
}

func (self *IPVSDriver) Print() {
    if self.ipvsClient == nil {
        fmt.Printf("Mock'd\n")
    } else if services, err := self.ipvsClient.ListServices(); err != nil {
        log.Fatalf("ipvs.ListServices: %v\n", err)
    } else {
        fmt.Printf("Proto                           Addr:Port\n")
        for _, service := range services {
            fmt.Printf("%-5v %30s:%-5d %s\n",
                service.Protocol,
                service.Addr, service.Port,
                service.SchedName,
            )

            if dests, err := self.ipvsClient.ListDests(service); err != nil {
                log.Fatalf("ipvs.ListDests: %v\n", err)
            } else {
                for _, dest := range dests {
                    fmt.Printf("%5s %30s:%-5d %v\n",
                        "",
                        dest.Addr, dest.Port,
                        dest.FwdMethod,
                    )
                }
            }
        }
    }
}
