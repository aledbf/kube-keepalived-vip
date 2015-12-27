package config
/*
 * Externally exposed types.
 */

type ServiceFrontend struct {
    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`
}

type ServiceBackend struct {
    IPv4    string  `json:"ipv4,omitempty"`
    IPv6    string  `json:"ipv6,omitempty"`
    TCP     uint16  `json:"tcp,omitempty"`
    UDP     uint16  `json:"udp,omitempty"`

    Weight  uint    `json:"weight,omitempty"`   // default: 10
}

type Route struct {
    // IPv4 prefix to match
    // empty for default match
    Prefix4     string

    // Override backend IPv4 address for ipvs
    Gateway4    string

    // Configure IPVS fwd-method to use for destination
    //  droute tunnel masq
    // Filter out backend if set to
    //  filter
    IpvsMethod  string
}

/*
 * Events when config changes
 */
type Action string

const (
    // Initialize configuration into a consistent state from Scan()
    NewConfig     Action   = "new"

    // Changes to active configuration from Sync()
    SetConfig     Action   = "set"
    DelConfig     Action   = "del"
)

// A Config has changed
type Event struct {
    Action      Action
    Config      Config
}

type Config interface {
    // Unique path for this config
    Path() string

    // JSON-encodeable value 
    Value() interface{}
}

/*
 * Configuration sources: where the config is coming from
 */
type ConfigSource string

const (
    // A configuration from a local file
    FileConfigSource ConfigSource = "file"

    // A configuration from Etcd
    EtcdConfigSource ConfigSource = "etcd"
)

/* Different config objects */

// Used when a new service directory is created or destroyed.
// May not necessarily be delivered when a new service is created; you can expect to directly get a ConfigService* event for a new service
// May be delievered with an empty ServiceName:"" if *all* services are to be deleted
type ConfigService struct {
    ServiceName     string
    ConfigSource    ConfigSource
}

type ConfigServiceFrontend struct {
    ServiceName     string

    Frontend        ServiceFrontend
    ConfigSource    ConfigSource
}

// May be delivered with an empty BackendName:"" if *all* service backends are to be deleted
type ConfigServiceBackend struct {
    ServiceName     string
    BackendName     string

    Backend         ServiceBackend
    ConfigSource    ConfigSource
}

type ConfigRoute struct {
    RouteName       string

    Route           Route
    ConfigSource    ConfigSource
}
