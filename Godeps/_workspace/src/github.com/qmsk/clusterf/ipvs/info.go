package ipvs

import (
    "fmt"
    "github.com/hkwi/nlgo"
)

/* Packed version number */
type Version uint32

func (version Version) String() string {
    return fmt.Sprintf("%d.%d.%d",
        (version >> 16) & 0xFF,
        (version >> 8)  & 0xFF,
        (version >> 0)  & 0xFF,
    )
}

type Info struct {
    Version     Version
    ConnTabSize uint32
}

func unpackInfo(attrs nlgo.AttrMap) (info Info, err error) {
    for _, attr := range attrs.Slice() {
        switch attr.Field() {
        case IPVS_INFO_ATTR_VERSION:        info.Version = (Version)(attr.Value.(nlgo.U32))
        case IPVS_INFO_ATTR_CONN_TAB_SIZE:  info.ConnTabSize = (uint32)(attr.Value.(nlgo.U32))
        }
    }

    return
}
