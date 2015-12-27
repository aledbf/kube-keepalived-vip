package ipvs

import (
    "fmt"
    "net"
    "github.com/hkwi/nlgo"
)

type FwdMethod uint32

func (self FwdMethod) String() string {
    switch value := (uint32(self) & IP_VS_CONN_F_FWD_MASK); value {
    case IP_VS_CONN_F_MASQ:
        return "masq"
    case IP_VS_CONN_F_LOCALNODE:
        return "localnode"
    case IP_VS_CONN_F_TUNNEL:
        return "tunnel"
    case IP_VS_CONN_F_DROUTE:
        return "droute"
    case IP_VS_CONN_F_BYPASS:
        return "bypass"
    default:
        return fmt.Sprintf("%#04x", value)
    }
}

func ParseFwdMethod(value string) (FwdMethod, error) {
    switch value {
    case "masq":
        return IP_VS_CONN_F_MASQ, nil
    case "tunnel":
        return IP_VS_CONN_F_TUNNEL, nil
    case "droute":
        return IP_VS_CONN_F_DROUTE, nil
    default:
        return 0, fmt.Errorf("Invalid FwdMethod: %s", value)
    }
}

type Dest struct {
    // id
    // TODO: IPVS_DEST_ATTR_ADDR_FAMILY
    Addr        net.IP
    Port        uint16

    // params
    FwdMethod   FwdMethod
    Weight      uint32
    UThresh     uint32
    LThresh     uint32

    // info
    ActiveConns     uint32
    InactConns      uint32
    PersistConns    uint32
}

// Acts as an unique identifier for the Service
func (self Dest) String() string {
    return fmt.Sprintf("%s:%d", self.Addr, self.Port)
}

func unpackDest(service Service, attrs nlgo.AttrMap) (Dest, error) {
    var dest Dest
    var addr []byte

    for _, attr := range attrs.Slice() {
        switch attr.Field() {
        case IPVS_DEST_ATTR_ADDR:       addr = ([]byte)(attr.Value.(nlgo.Binary))
        case IPVS_DEST_ATTR_PORT:       dest.Port = unpackPort(attr.Value.(nlgo.U16))
        case IPVS_DEST_ATTR_FWD_METHOD: dest.FwdMethod = (FwdMethod)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_WEIGHT:     dest.Weight = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_U_THRESH:   dest.UThresh = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_L_THRESH:   dest.LThresh = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_ACTIVE_CONNS:   dest.ActiveConns = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_INACT_CONNS:    dest.InactConns = (uint32)(attr.Value.(nlgo.U32))
        case IPVS_DEST_ATTR_PERSIST_CONNS:  dest.PersistConns = (uint32)(attr.Value.(nlgo.U32))
        }
    }

    if addrIP, err := unpackAddr(addr, service.Af); err != nil {
        return dest, fmt.Errorf("ipvs:Dest.unpack: addr: %s", err)
    } else {
        dest.Addr = addrIP
    }

    return dest, nil
}

// Dump Dest as nl attrs, using the Af of the corresponding Service.
// If full, includes Dest setting attrs, otherwise only identifying attrs.
func (self *Dest) attrs(service *Service, full bool) nlgo.AttrSlice {
    var attrs nlgo.AttrSlice

    attrs = append(attrs,
        nlattr(IPVS_DEST_ATTR_ADDR, packAddr(service.Af, self.Addr)),
        nlattr(IPVS_DEST_ATTR_PORT, packPort(self.Port)),
    )

    if full {
        attrs = append(attrs,
            nlattr(IPVS_DEST_ATTR_FWD_METHOD,   nlgo.U32(self.FwdMethod)),
            nlattr(IPVS_DEST_ATTR_WEIGHT,       nlgo.U32(self.Weight)),
            nlattr(IPVS_DEST_ATTR_U_THRESH,     nlgo.U32(self.UThresh)),
            nlattr(IPVS_DEST_ATTR_L_THRESH,     nlgo.U32(self.LThresh)),
        )
    }

    return attrs
}

