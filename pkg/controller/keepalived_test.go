package controller

import (
	"os"
	"path"
	"runtime"
	"testing"

	utildbus "k8s.io/kubernetes/pkg/util/dbus"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	utilexec "k8s.io/utils/exec"
)

func TestKeepalived_GenerateCfg(t *testing.T) {
	keepalivedConfigWant := "\n\n\nglobal_defs {\n  vrrp_version 3\n  vrrp_iptables KUBE-KEEPALIVED-VIP\n}\n\n\n#Check " +
		"if the VIP list is empty\n\n\n\n\n\n\nvrrp_instance vips {\n  state BACKUP\n  interface eth0\n  virtual_router_id " +
		"100\n  priority 101\n  nopreempt\n  advert_int 1\n\n  track_interface {\n    eth0\n  }\n\n  \n\n  virtual_ipaddress " +
		"{ \n    192.168.0.1\n  }\n\n  notify /keepalived-check.sh\n\n\n\n}\n\nvrrp_instance vips {\n  state BACKUP\n  " +
		"interface eth1\n  virtual_router_id 101\n  priority 101\n  nopreempt\n  advert_int 1\n\n  track_interface {\n  " +
		"  eth1\n  }\n\n  \n\n  virtual_ipaddress { \n    192.168.1.1\n  }\n\n  notify /keepalived-check.sh\n\n\n\n}" +
		"\n\n\n\n\n\n# Service: service-1\nvirtual_server 192.168.0.1 10001 {\n  delay_loop 5\n  lvs_sched wlc\n  lvs_method " +
		"NAT\n  persistence_timeout 1800\n  protocol TCP\n\n  \n  real_server 10.0.0.1 1001 {\n    weight 1\n    TCP_CHECK " +
		"{\n      connect_port 1001\n      connect_timeout 3\n    }\n  }\n  \n}\n\n\n\n# Service: service-2\nvirtual_server " +
		"192.168.0.1 10002 {\n  delay_loop 5\n  lvs_sched wlc\n  lvs_method NAT\n  persistence_timeout 1800\n  protocol " +
		"TCP\n\n  \n  real_server 10.0.0.1 1002 {\n    weight 1\n    TCP_CHECK {\n      connect_port 1002\n      " +
		"connect_timeout 3\n    }\n  }\n  \n}\n\n\n\n# Service: service-3\nvirtual_server 192.168.1.1 10001 {\n  " +
		"delay_loop 5\n  lvs_sched wlc\n  lvs_method NAT\n  persistence_timeout 1800\n  protocol TCP\n\n  \n  real_server " +
		"10.0.1.1 1001 {\n    weight 1\n    TCP_CHECK {\n      connect_port 1001\n      connect_timeout 3\n    }\n  }\n  \n}" +
		"\n\n\n\n#End if vip list is empty\n\n\n"

	svcs := []vip{
		{
			Name:      "service-1",
			IP:        "192.168.0.1",
			Port:      10001,
			Protocol:  "TCP",
			LVSMethod: "NAT",
			iface:     "eth0",
			Backends: []service{
				{IP: "10.0.0.1", Port: 1001},
			},
		},
		{
			Name:      "service-2",
			IP:        "192.168.0.1",
			Port:      10002,
			Protocol:  "TCP",
			LVSMethod: "NAT",
			iface:     "eth0",
			Backends: []service{
				{IP: "10.0.0.1", Port: 1002},
			},
		},
		{
			Name:      "service-3",
			IP:        "192.168.1.1",
			Port:      10001,
			Protocol:  "TCP",
			LVSMethod: "NAT",
			iface:     "eth1",
			Backends: []service{
				{IP: "10.0.1.1", Port: 1001},
			},
		},
	}

	execer := utilexec.New()
	dbus := utildbus.New()
	iptInterface := utiliptables.New(execer, dbus, utiliptables.ProtocolIpv4)

	k := &keepalived{
		iface:      "ethDefault",
		ip:         "10.1.1.1",
		netmask:    24,
		nodes:      []string{"10.1.1.1", "10.1.1.2"},
		neighbors:  []string{"10.1.1.2"},
		priority:   101,
		useUnicast: false,
		ipt:        iptInterface,
		vrid:       100,
		proxyMode:  false,
	}

	_, filename, _, _ := runtime.Caller(0)
	err := os.Chdir(path.Dir(filename) + "/../../rootfs")

	if err := k.loadTemplates(); err != nil {
		t.Fatalf("GenerateCfg returned error: %v", err)
	}

	keepalivedConfig, _, err := k.GenerateCfg(svcs)
	if err != nil {
		t.Fatalf("GenerateCfg returned error: %v", err)
	}

	if keepalivedConfigWant != string(keepalivedConfig) {
		t.Errorf("Invalid config generated: \n === want === \n%q\n=== got ===\n%q", keepalivedConfigWant, keepalivedConfig)
	}
}
