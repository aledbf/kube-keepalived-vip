package config

import (
    "github.com/coreos/go-etcd/etcd"
    etcdError "github.com/coreos/etcd/error"
    "fmt"
    "log"
    "strings"
)

type EtcdConfig struct {
    Machines    string
    Prefix      string
}

type Etcd struct {
    config      EtcdConfig
    client      *etcd.Client

    syncIndex   uint64
    watchChan   chan Event
}

func (self *Etcd) String() string {
    return fmt.Sprintf("%s%s", self.config.Machines, self.config.Prefix)
}

/*
 * Open etcd session
 */
func (self EtcdConfig) Open() (*Etcd, error) {
    e := &Etcd{config: self}

    machines := strings.Split(self.Machines, ",")

    e.client = etcd.NewClient(machines)

    return e, nil
}

/*
 * Initialize state in etcd
 */
func (self *Etcd) Init() error {
    if response, err := self.client.CreateDir(self.config.Prefix, 0); err != nil {
        return err
    } else {
        self.syncIndex = response.Node.CreatedIndex
    }

    return nil
}

/*
 * Synchronize current state in etcd.
 *
 * Does a recursive get on the complete /clusterf tree in etcd, and builds the services state from it.
 *
 * Stores the current etcd-index from the snapshot in .syncIndex, so that .Sync() can be used to continue updating any changes.
 */
func (self *Etcd) Scan() ([]Config, error) {
    response, err := self.client.Get(self.config.Prefix, false, /* recursive */ true)

    if err != nil {
        if etcdErr, ok := err.(*etcd.EtcdError); ok {
            if etcdErr.ErrorCode == etcdError.EcodeKeyNotFound {
                // create directory instead
                return nil, self.Init()
            }
        }

        return nil, err
    }

    if response.Node.Dir != true {
        return nil, fmt.Errorf("--etcd-prefix=%s is not a directory", response.Node.Key)
    }

    // the tree root's ModifiedTime may be a long long time in the past, so we can't want to use that for waits
    // we assume this enough to ensure atomic sync with .Watch() on the same tree..
    self.syncIndex = response.EtcdIndex

    // scan, collect and return
    var configs []Config
    err = self.scan(response.Node, func (config Config) {
        configs = append(configs, config)
    })
    return configs, err
}

// Scan through the recursive /clusterf node to return ConfigItem's
func (self *Etcd) scan(node *etcd.Node, configHandler func(Config)) error {
    // decode etcd path into config tree path
    path := node.Key

    if !strings.HasPrefix(node.Key, self.config.Prefix) {
        return fmt.Errorf("node outside tree: %s", node.Key)
    }

    path = strings.TrimPrefix(path, self.config.Prefix)
    path = strings.Trim(path, "/")

    // match
    configNode := Node{
        Path:   path,
        IsDir:  node.Dir,
        Value:  node.Value,
        Source: EtcdConfigSource,
    }

    if config, err := syncConfig(configNode); err != nil {
        log.Printf("config:etcd.scan %s: %v\n", node.Key, err)
    } else if config == nil {

    } else {
        log.Printf("config:etcd.scan %s: %#v\n", node.Key, config)

        configHandler(config)
    }

    // recurse
    for _, childNode := range node.Nodes {
        if err := self.scan(childNode, configHandler); err != nil {
            return err
        }
    }

    return nil
}

/*
 * Watch for changes in etcd
 *
 * Sends any changes on the returned channel.
 */
func (self *Etcd) Sync() chan Event {
    if self.watchChan == nil {
        // kick off new goroutine to handle initial services and updates
        self.watchChan = make(chan Event)

        go self.watch()
    }

    return self.watchChan
}

// Watch etcd for changes, and sync them
func (self *Etcd) watch() {
    defer close(self.watchChan)

    for {
        response, err := self.client.Watch(self.config.Prefix, self.syncIndex + 1, true, nil, nil)
        if err != nil {
            log.Printf("config:etcd.watch %s @ %d: %s\n", self.config.Prefix, self.syncIndex + 1, err)
            break
        } else {
            self.syncIndex = response.Node.ModifiedIndex
        }

        if response.PrevNode != nil {
            log.Printf("config:etcd.watch: %s %+v <- %+v\n", response.Action, response.Node, response.PrevNode)
        } else {
            log.Printf("config:etcd.watch: %s %+v\n", response.Action, response.Node)
        }

        // sync to update services state and generate watchEvent()'s
        if event, err := self.sync(response.Action, response.Node); err != nil {
            log.Printf("config:etcd.sync: %s\n", err)
            continue
        } else if event != nil {
            self.watchChan <- *event
        }
    }
}

// Handle changed node
func (self *Etcd) sync(action string, node *etcd.Node) (*Event, error) {
    // decode action
    eventAction := func()Action{ switch action {
    case "create", "set":
        return SetConfig

    case "delete", "expire":
        return DelConfig

    default:
        panic(fmt.Errorf("Unknown etcd action: %s", action))

    } }()


    // decode etcd path into config tree path
    path := node.Key

    if strings.HasPrefix(path, self.config.Prefix) {
        path = strings.TrimPrefix(path, self.config.Prefix)
    } else {
        return nil, fmt.Errorf("path outside tree: %s", path)
    }
    path = strings.Trim(path, "/")

    // match
    eventNode := Node{
        Path:   path,
        IsDir:  node.Dir,
        Value:  node.Value,
    }

    if event, err := syncEvent(eventAction, eventNode); err != nil {
        log.Printf("config:Etcd.sync %s %s: %v\n", action, node.Key, err)
        return nil, err
    } else if event == nil {
        return nil, nil
    } else {
        log.Printf("config:Etcd.sync %s %s: %#v\n", action, node.Key, event)
        return event, err
    }
}

func (self *Etcd) path(parts ...string) string {
    return strings.Join(append([]string{self.config.Prefix}, parts...), "/")
}

// Publish a config into etcd
func (self *Etcd) Publish(config Config) error {
    var ttl uint64 = 0

    if node, err := makeNode(config); err != nil {
        return err
    } else if _, err := self.client.Set(self.path(node.Path), node.Value, ttl); err != nil {
        return err
    } else {
        return nil
    }
}

// Retract a config from etcd
func (self *Etcd) Retract(config Config) error {
    if _, err := self.client.Delete(self.path(config.Path()), false); err != nil {
        return err
    } else {
        return nil
    }
}
