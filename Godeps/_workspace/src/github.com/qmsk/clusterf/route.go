package clusterf

import (
    "github.com/qmsk/clusterf/config"
    "fmt"
    "github.com/qmsk/clusterf/ipvs"
    "net"
)

type Routes map[string]*Route

func makeRoutes() Routes {
    return Routes(make(map[string]*Route))
}

func (self Routes) get(name string) *Route {
    if route, exists := self[name]; exists {
        return route
    } else {
        route := &Route{
            Name: name,
        }
        self[name] = route

        return route
    }
}

func (self Routes) del(name string) {
    delete(self, name)
}

// Return most-specific matching route for given IPv4/IPv6 IP
func (self Routes) Lookup(ip net.IP) *Route {
    var matchRoute *Route
    var matchLength int = 0

    for _, route := range self {
        if match, routeLength := route.match(ip); !match {

        } else if matchRoute == nil || routeLength > matchLength {
            matchRoute = route
        }
    }

    return matchRoute
}



type Route struct {
    Name        string

    // default -> nil
    Prefix4     *net.IPNet

    // attributes
    Gateway4        net.IP
    ipvs_fwdMethod  *ipvs.FwdMethod
    ipvs_filter     bool
}

func (self *Route) config(action config.Action, routeConfig config.Route) error {
    if routeConfig.Prefix4 == "" {
        self.Prefix4 = nil // default
    } else if _, prefix4, err := net.ParseCIDR(routeConfig.Prefix4); err != nil {
        return fmt.Errorf("Invalid Prefix4: %s", routeConfig.Prefix4)
    } else {
        self.Prefix4 = prefix4
    }

    if routeConfig.Gateway4 == "" {
        self.Gateway4 = nil
    } else if gateway4 := net.ParseIP(routeConfig.Gateway4).To4(); gateway4 == nil {
        return fmt.Errorf("Invalid Gateway4: %s", routeConfig.Gateway4)
    } else {
        self.Gateway4 = gateway4
    }

    if routeConfig.IpvsMethod == "" {
        self.ipvs_filter = false
        self.ipvs_fwdMethod = nil
    } else if routeConfig.IpvsMethod == "filter" {
        self.ipvs_filter = true
        self.ipvs_fwdMethod = nil
    } else if fwdMethod, err := ipvs.ParseFwdMethod(routeConfig.IpvsMethod); err != nil {
        return err
    } else {
        self.ipvs_filter = false
        self.ipvs_fwdMethod = &fwdMethod
    }

    return nil
}

// Match given ip within our prefix
// Returns true if matches, with the length of the matching prefix
// Returns false otherwise
func (self *Route) match(ip net.IP) (match bool, length int) {
    if ip4 := ip.To4(); ip4 == nil {

    } else if self.Prefix4 == nil {
        // default match
        return true, 0

    } else if !self.Prefix4.Contains(ip4) {

    } else {
        prefixLength, _:= self.Prefix4.Mask.Size()

        return true, prefixLength
    }

    return false, 0
}
