package ipvs

import (
    "encoding/binary"
    "bytes"
    "fmt"
    "net"
    "github.com/hkwi/nlgo"
    "syscall"
)

// Helper to build an nlgo.Attr
func nlattr (typ uint16, value nlgo.NlaValue) nlgo.Attr {
    return nlgo.Attr{Header: syscall.NlAttr{Type: typ}, Value: value}
}

// Helpers for struct <-> nlgo.Binary
func unpack(value nlgo.Binary, out interface{}) error {
    return binary.Read(bytes.NewReader(([]byte)(value)), binary.BigEndian, out)
}

func pack (in interface{}) nlgo.Binary {
    var buf bytes.Buffer

    if err := binary.Write(&buf, binary.BigEndian, in); err != nil {
        panic(err)
    }

    return nlgo.Binary(buf.Bytes())
}

// Helpers for net.IP <-> nlgo.Binary
func unpackAddr (value nlgo.Binary, af Af) (net.IP, error) {
    buf := ([]byte)(value)
    size := 0

    switch af {
    case syscall.AF_INET:       size = 4
    case syscall.AF_INET6:      size = 16
    default:
        return nil, fmt.Errorf("ipvs: unknown af=%d addr=%v", af, buf)
    }

    if size > len(buf) {
        return nil, fmt.Errorf("ipvs: short af=%d addr=%v", af, buf)
    }

    return (net.IP)(buf[:size]), nil
}

func packAddr (af Af, addr net.IP) nlgo.Binary {
    var ip net.IP

    switch af {
        case syscall.AF_INET:   ip = addr.To4()
        case syscall.AF_INET6:  ip = addr.To16()
        default:
            panic(fmt.Errorf("ipvs:packAddr: unknown af=%d addr=%v", af, addr))
    }

    if ip == nil {
        panic(fmt.Errorf("ipvs:packAddr: invalid af=%d addr=%v", af, addr))
    }

    return (nlgo.Binary)(ip)
}

// Helpers for uint16 port <-> nlgo.U16
func htons (value uint16) uint16 {
    return ((value & 0x00ff) << 8) | ((value & 0xff00) >> 8)
}
func ntohs (value uint16) uint16 {
    return ((value & 0x00ff) << 8) | ((value & 0xff00) >> 8)
}

func unpackPort (val nlgo.U16) uint16 {
    return ntohs((uint16)(val))
}
func packPort (port uint16) nlgo.U16 {
    return nlgo.U16(htons(port))
}
