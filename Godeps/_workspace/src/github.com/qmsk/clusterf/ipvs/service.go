package ipvs

import (
    "fmt"
    "net"
    "github.com/hkwi/nlgo"
    "syscall"
)

type Flags struct {
    Flags   uint32
    Mask    uint32
}

type Af uint16

func (self Af) String() string {
    switch self {
    case syscall.AF_INET:
        return "inet"
    case syscall.AF_INET6:
        return "inet6"
    default:
        return fmt.Sprintf("%d", self)
    }
}

type Protocol uint16

func (self Protocol) String() string {
    switch self {
    case syscall.IPPROTO_TCP:
        return "tcp"
    case syscall.IPPROTO_UDP:
        return "udp"
    case syscall.IPPROTO_SCTP:
        return "sctp"
    default:
        return fmt.Sprintf("%d", self)
    }
}

type Service struct {
    // id
    Af          Af
    Protocol    Protocol
    Addr        net.IP
    Port        uint16
    FwMark      uint32

    // params
    SchedName   string
    Flags       Flags
    Timeout     uint32
    Netmask     uint32
}

// Acts as an unique identifier for the Service
func (self Service) String() string {
    if self.FwMark == 0 {
        return fmt.Sprintf("%v+%v://%s:%d", self.Af, self.Protocol, self.Addr, self.Port)
    } else {
        return fmt.Sprintf("%s+fwmark://%d", self.Af, self.FwMark)
    }
}

func unpackService(attrs nlgo.AttrMap) (Service, error) {
    var service Service

    var addr nlgo.Binary
    var flags nlgo.Binary

    for _, attr := range attrs.Slice() {
        switch attr.Field() {
        case IPVS_SVC_ATTR_AF:          service.Af = (Af)(attr.Value.(nlgo.U16))
        case IPVS_SVC_ATTR_PROTOCOL:    service.Protocol = (Protocol)(attr.Value.(nlgo.U16))
        case IPVS_SVC_ATTR_ADDR:        addr = attr.Value.(nlgo.Binary)
        case IPVS_SVC_ATTR_PORT:        service.Port = unpackPort(attr.Value.(nlgo.U16))
        case IPVS_SVC_ATTR_FWMARK:      service.FwMark = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_SVC_ATTR_SCHED_NAME:  service.SchedName = (string)(attr.Value.(nlgo.NulString))
        case IPVS_SVC_ATTR_FLAGS:       flags = attr.Value.(nlgo.Binary)
        case IPVS_SVC_ATTR_TIMEOUT:     service.Timeout = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_SVC_ATTR_NETMASK:     service.Netmask = (uint32)(attr.Value.(nlgo.U32))
        }
    }

    if addrIP, err := unpackAddr(addr, service.Af); err != nil {
        return service, fmt.Errorf("ipvs:Service.unpack: addr: %s", err)
    } else {
        service.Addr = addrIP
    }

    if err := unpack(flags, &service.Flags); err != nil {
        return service, fmt.Errorf("ipvs:Service.unpack: flags: %s", err)
    }

    return service, nil
}

// Pack Service to a set of nlattrs.
// If full is given, include service settings, otherwise only the identifying fields are given.
func (self *Service) attrs(full bool) nlgo.AttrSlice {
    var attrs nlgo.AttrSlice

    if self.FwMark != 0 {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_AF, nlgo.U16(self.Af)),
            nlattr(IPVS_SVC_ATTR_FWMARK, nlgo.U32(self.FwMark)),
        )
    } else if self.Protocol != 0 && self.Addr != nil && self.Port != 0 {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_AF, nlgo.U16(self.Af)),
            nlattr(IPVS_SVC_ATTR_PROTOCOL, nlgo.U16(self.Protocol)),
            nlattr(IPVS_SVC_ATTR_ADDR, packAddr(self.Af, self.Addr)),
            nlattr(IPVS_SVC_ATTR_PORT, packPort(self.Port)),
        )
    } else {
        panic("Incomplete service id fields")
    }

    if full {
        attrs = append(attrs,
            nlattr(IPVS_SVC_ATTR_SCHED_NAME,    nlgo.NulString(self.SchedName)),
            nlattr(IPVS_SVC_ATTR_FLAGS,         pack(&self.Flags)),
            nlattr(IPVS_SVC_ATTR_TIMEOUT,       nlgo.U32(self.Timeout)),
            nlattr(IPVS_SVC_ATTR_NETMASK,       nlgo.U32(self.Netmask)),
        )
    }

    return attrs
}

