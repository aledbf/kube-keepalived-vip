package main

import (
    "github.com/qmsk/clusterf/config"
    "github.com/qmsk/clusterf"
    "flag"
    "log"
    "os"
)

var (
    filesConfig config.FilesConfig
    etcdConfig  config.EtcdConfig
    ipvsConfig  clusterf.IpvsConfig
    ipvsConfigPrint bool
    advertiseRouteConfig     config.ConfigRoute
    filterEtcdRoutes    bool
)

func init() {
    flag.StringVar(&filesConfig.Path, "config-path", "",
        "Local config tree")

    flag.StringVar(&etcdConfig.Machines, "etcd-machines", "http://127.0.0.1:2379",
        "Client endpoint for etcd")
    flag.StringVar(&etcdConfig.Prefix, "etcd-prefix", "/clusterf",
        "Etcd tree prefix")

    flag.BoolVar(&ipvsConfig.Debug, "ipvs-debug", false,
        "IPVS debugging")
        flag.BoolVar(&ipvsConfigPrint, "ipvs-print", false,
        "Dump initial IPVS config")
    flag.StringVar(&ipvsConfig.FwdMethod, "ipvs-fwd-method", "masq",
        "IPVS Forwarding method: masq tunnel droute")
    flag.StringVar(&ipvsConfig.SchedName, "ipvs-sched-name", clusterf.IPVS_SCHED_NAME,
        "IPVS Service Scheduler")

    flag.StringVar(&advertiseRouteConfig.RouteName, "advertise-route-name", "",
        "Advertise route by name")
    flag.StringVar(&advertiseRouteConfig.Route.Prefix4, "advertise-route-prefix4", "",
        "Advertise route for prefix")
    flag.StringVar(&advertiseRouteConfig.Route.Gateway4, "advertise-route-gateway4", "",
        "Advertise route via gateway")
    flag.StringVar(&advertiseRouteConfig.Route.IpvsMethod, "advertise-route-ipvs-method", "",
        "Advertise route ipvs-fwd-method")

    flag.BoolVar(&filterEtcdRoutes, "filter-etcd-routes", false,
        "Filter out etcd routes")
}

// Apply filtering for etcdConfig sourced Config's
// Returns false if config should be filtered
func filterConfigEtcd(baseConfig config.Config) bool {
    switch baseConfig.(type) {
    case *config.ConfigRoute:
        if filterEtcdRoutes {
            return true
        }
    }

    return false
}

func main() {
    flag.Parse()

    if len(flag.Args()) > 0 {
        flag.Usage()
        os.Exit(1)
    }

    // setup
    services := clusterf.NewServices()

    // config
    var configFiles *config.Files
    var configEtcd *config.Etcd

    if filesConfig.Path != "" {
        if files, err := filesConfig.Open(); err != nil {
            log.Fatalf("config:Files.Open: %s\n", err)
        } else {
            configFiles = files

            log.Printf("config:Files.Open: %s\n", configFiles)
        }

        if configs, err := configFiles.Scan(); err != nil {
            log.Fatalf("config:Files.Scan: %s\n", err)
        } else {
            log.Printf("config:Files.Scan: %d configs\n", len(configs))

            // iterate initial set of services
            for _, cfg := range configs {
                services.NewConfig(cfg)
            }
        }
    }

    if etcdConfig.Prefix != "" {
        if etcd, err := etcdConfig.Open(); err != nil {
            log.Fatalf("config:etcd.Open: %s\n", err)
        } else {
            configEtcd = etcd

            log.Printf("config:etcd.Open: %s\n", configEtcd)
        }

        if configs, err := configEtcd.Scan(); err != nil {
            log.Fatalf("config:Etcd.Scan: %s\n", err)
        } else {
            log.Printf("config:Etcd.Scan: %d configs\n", len(configs))

            // iterate initial set of services
            for _, cfg := range configs {
                if filterConfigEtcd(cfg) {
                    continue
                }

                services.NewConfig(cfg)
            }
        }
    }

    // sync
    if ipvsDriver, err := services.SyncIPVS(ipvsConfig); err != nil {
        log.Fatalf("SyncIPVS: %s\n", err)
    } else {
        if ipvsConfigPrint {
            ipvsDriver.Print()
        }
    }

    // advertise
    if advertiseRouteConfig.RouteName == "" || configEtcd == nil {

    } else if err := configEtcd.Publish(advertiseRouteConfig); err != nil {
        log.Fatalf("config:Etcd.Publish advertiseRoute %#v: %v\n", advertiseRouteConfig, err)
    } else {
        log.Printf("config:Etcd.Publish advertiseRoute %#v\n", advertiseRouteConfig)
    }

    if configEtcd != nil {
        // read channel for changes
        log.Printf("config:Etcd.Sync...\n")

        for event := range configEtcd.Sync() {
            if filterConfigEtcd(event.Config) {
                continue
            }

            log.Printf("config.Sync: %+v\n", event)

            services.ConfigEvent(event)
        }
    }


    log.Printf("Exit\n")
}
