package config

import (
    "fmt"
    "log"
    "regexp"
    "testing"
)

func loadBackend (t *testing.T, value string) ServiceBackend {
    node := Node{Source:"test", Path: "services/test/backends/test", Value: value}

    if backend, err := node.loadServiceBackend(); err != nil {
        t.Fatalf("ServiceBackend.loadEtcd(%v): %s", value, err)
        return ServiceBackend{ } // XXX
    } else {
        return backend
    }
}

func TestBackendLoad (t *testing.T) {
    simple := loadBackend(t, "{\"ipv4\": \"127.0.0.1\"}")

    if simple.IPv4 != "127.0.0.1" {
        t.Error("%v.IPv4 != 127.0.0.1", simple)
    }
}

var testSync = []struct {
    action  Action
    node    Node
    event   Event
    error   string
}{
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"", Value:"haha"},
        error: "Ignore unknown node",
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services", Value:"haha"},
        error: "Ignore unknown node",
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"wtf", Value:"haha"},
        error: "Ignore unknown node",
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"wtf", IsDir:true},
        error: "Ignore unknown node",
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/wtf/frontend", IsDir:true},
        error: "Ignore unknown service wtf node",
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/wtf/backends/test", IsDir:true},
        error: "Ignore unknown service wtf backends node",
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/wtf/backends/test/three", Value: "3"},
        error: "Ignore unknown service wtf backends node",
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/wtf/asdf", Value: "quux"},
        error: "Ignore unknown service wtf node",
    },

    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/test/frontend", Value:"not json"},
        error: "service test frontend: invalid character 'o' in literal null",
    },

    {
        action: NewConfig,
        node: Node{Source:"test", Path:"", IsDir:true},
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services", IsDir:true},
        event: Event{Action: NewConfig, Config: &ConfigService{
            ConfigSource: "test",
            ServiceName: "",
        }},
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/test", IsDir:true},
        event: Event{Action: NewConfig, Config: &ConfigService{
            ConfigSource: "test",
            ServiceName: "test",
        }},
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/test/frontend", Value: "{\"ipv4\": \"127.0.0.1\", \"tcp\": 8080}"},
        event: Event{Action: NewConfig, Config: &ConfigServiceFrontend{
            ConfigSource: "test",
            ServiceName: "test",
            Frontend:    ServiceFrontend{IPv4: "127.0.0.1", TCP: 8080},
        }},
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/test/backends", IsDir:true},
        event: Event{Action: NewConfig, Config: &ConfigServiceBackend{
            ConfigSource: "test",
            ServiceName: "test",
            BackendName: "",
        }},
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/test/backends/test1", Value: "{\"ipv4\": \"127.0.0.1\", \"tcp\": 8081}"},
        event: Event{Action: NewConfig, Config: &ConfigServiceBackend{
            ConfigSource: "test",
            ServiceName: "test",
            BackendName: "test1",
            Backend:     ServiceBackend{IPv4: "127.0.0.1", TCP: 8081},
        }},
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/test/backends/test2", Value: "{\"ipv4\": \"127.0.0.1\", \"tcp\": 8082}"},
        event: Event{Action: NewConfig, Config: &ConfigServiceBackend{
            ConfigSource: "test",
            ServiceName: "test",
            BackendName: "test2",
            Backend:     ServiceBackend{IPv4: "127.0.0.1", TCP: 8082},
        }},
    },
    {
        action: NewConfig,
        node: Node{Source:"test", Path:"services/test6/frontend", Value: "{\"ipv6\": \"2001:db8::1\", \"tcp\": 8080}"},
        event: Event{Action: NewConfig, Config: &ConfigServiceFrontend{
            ConfigSource: "test",
            ServiceName: "test6",
            Frontend:    ServiceFrontend{IPv6: "2001:db8::1", TCP: 8080},
        }},
    },

    {
        action: DelConfig,
        node: Node{Source:"test", Path:"services/test3/backends/test1"},
        event: Event{Action: DelConfig, Config: &ConfigServiceBackend{
            ConfigSource: "test",
            ServiceName: "test3",
            BackendName: "test1",
        }},
    },
    {
        action: DelConfig,
        node: Node{Source:"test", Path:"services/test3/backends", IsDir:true},
        event: Event{Action: DelConfig, Config: &ConfigServiceBackend{
            ConfigSource: "test",
            ServiceName: "test3",
            BackendName: "",
        }},
    },
    {
        action: DelConfig,
        node: Node{Source:"test", Path:"services/test3", IsDir:true},
        event: Event{Action: DelConfig, Config: &ConfigService{
            ConfigSource: "test",
            ServiceName: "test3",
        }},
    },
    {
        action: DelConfig,
        node: Node{Source:"test", Path:"services/test", IsDir:true},
        event: Event{Action: DelConfig, Config: &ConfigService{
            ConfigSource: "test",
            ServiceName: "test",
        }},
    },
    {
        action: DelConfig,
        node: Node{Source:"test", Path:"services", IsDir:true},
        event: Event{Action: DelConfig, Config: &ConfigService{
            ConfigSource: "test",
            ServiceName: "",
        }},
    },
}

func TestSync(t *testing.T) {
    for _, testCase := range testSync {
        log.Printf("--- %+v\n", testCase)
        event, err := syncEvent(testCase.action, testCase.node)

        if err != nil {
            if testCase.error == "" {
                t.Errorf("error %+v: error %s", testCase, err)
            } else if !regexp.MustCompile(testCase.error).MatchString(err.Error()) {
                t.Errorf("fail %+v: error: %s", testCase, err)
            }
        } else if testCase.error != "" {
            t.Errorf("fail %+v: error nil", testCase)
        }

        if event == nil && testCase.event.Action == "" {

        } else if event == nil && testCase.event.Action != "" {
            t.Errorf("fail %+v: missing event %+v", testCase, testCase.event)
        } else if event != nil && testCase.event.Action == "" {
            t.Errorf("fail %+v: extra event %+v", testCase, event)
        } else {
            if event.Action != testCase.event.Action {
                t.Errorf("fail %+v: event %+v action", testCase, event)
            }

            // XXX: lawlz comparing interface{} structs for equality
            if fmt.Sprintf("%#v", event.Config) != fmt.Sprintf("%#v", testCase.event.Config) {
                t.Errorf("fail %+v: event %#v config", testCase, event.Config)
            }
        }
    }
}
