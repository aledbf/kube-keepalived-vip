/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/util/iptables"
	k8sexec "k8s.io/utils/exec"
)

const (
	iptablesChain   = "KUBE-KEEPALIVED-VIP"
	keepalivedCfg   = "/etc/keepalived/keepalived.conf"
	haproxyCfg      = "/etc/haproxy/haproxy.cfg"
	keepalivedPid   = "/var/run/keepalived.pid"
	keepalivedState = "/var/run/keepalived.state"
	vrrpPid         = "/var/run/vrrp.pid"
	vridBase        = 100
)

var (
	keepalivedTmpl = "keepalived.tmpl"
	haproxyTmpl    = "haproxy.tmpl"
)

type keepalived struct {
	iface          string
	ip             string
	netmask        int
	priority       int
	nodes          []string
	neighbors      []string
	useUnicast     bool
	started        bool
	vips           []string
	keepalivedTmpl *template.Template
	haproxyTmpl    *template.Template
	cmd            *exec.Cmd
	ipt            iptables.Interface
	vrid           int
	proxyMode      bool
}

// WriteCfg creates a new keepalived configuration file.
// In case of an error with the generation it returns the error
func (k *keepalived) WriteCfg(svcs []vip) error {
	keepalivedConfig, haproxyConfig, err := k.GenerateCfg(svcs)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(keepalivedCfg, keepalivedConfig, 0644)
	if err != nil {
		return err
	}

	if haproxyConfig != nil {
		err = ioutil.WriteFile(haproxyCfg, haproxyConfig, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func (k *keepalived) GenerateCfg(svcs []vip) (keepalivedConfig, haproxyConfig []byte, err error) {
	keepalivedBuffer := new(bytes.Buffer)
	haproxyBuffer := new(bytes.Buffer)

	k.vips = getVIPs(svcs)

	ifaces, err := getIfaces(svcs)
	if err != nil {
		return nil, nil, err
	}

	conf := make(map[string]interface{})
	conf["iptablesChain"] = iptablesChain
	conf["myIP"] = k.ip
	conf["netmask"] = k.netmask
	conf["svcs"] = svcs
	conf["vips"] = k.vips
	conf["ifaces"] = ifaces
	conf["nodes"] = k.neighbors
	conf["priority"] = k.priority
	conf["useUnicast"] = k.useUnicast
	conf["vrid"] = k.vrid
	conf["proxyMode"] = k.proxyMode
	conf["vipIsEmpty"] = len(k.vips) == 0

	if glog.V(2) {
		b, _ := json.Marshal(conf)
		glog.Infof("%v", string(b))
	}

	err = k.keepalivedTmpl.Execute(keepalivedBuffer, conf)
	if err != nil {
		return nil, nil, fmt.Errorf("unexpected error creating keepalived.cfg: %v", err)
	}

	if k.proxyMode {
		err = k.haproxyTmpl.Execute(haproxyBuffer, conf)
		if err != nil {
			return nil, nil, fmt.Errorf("unexpected error creating haproxy.cfg: %v", err)
		}
	}

	return keepalivedBuffer.Bytes(), haproxyBuffer.Bytes(), nil
}

// getVIPs returns a list of the virtual IP addresses to be used in keepalived
// without duplicates (a service can use more than one port)
func getVIPs(svcs []vip) []string {
	result := []string{}
	for _, svc := range svcs {
		result = appendIfMissing(result, svc.IP)
	}

	return result
}

type Iface struct {
	Name     string
	Vrid     int
	Services []vip
	Ips      []string
}

type IfaceSlice []*Iface

func (c IfaceSlice) Len() int           { return len(c) }
func (c IfaceSlice) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c IfaceSlice) Less(i, j int) bool { return c[i].Name < c[j].Name }

// getVipIfaces returns a map{IP:iface, } from VIP list.
func getIfaces(svcs []vip) ([]*Iface, error) {
	ifaceByName := make(map[string]*Iface)
	for _, svc := range svcs {
		if _, ok := ifaceByName[svc.iface]; !ok {
			ifaceByName[svc.iface] = &Iface{Name: svc.iface}
		}

		ifaceByName[svc.iface].Services = append(ifaceByName[svc.iface].Services, svc)
		ifaceByName[svc.iface].Ips = appendIfMissing(ifaceByName[svc.iface].Ips, svc.IP)
	}

	var result IfaceSlice
	for _, iface := range ifaceByName {
		result = append(result, iface)
	}

	sort.Sort(result)

	for i, iface := range result {
		iface.Vrid = vridBase + i
	}

	return result, nil
}

// Start starts a keepalived process in foreground.
// In case of any error it will terminate the execution with a fatal error
func (k *keepalived) Start() {
	ae, err := k.ipt.EnsureChain(iptables.TableFilter, iptables.Chain(iptablesChain))
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}
	if ae {
		glog.V(2).Infof("chain %v already existed", iptablesChain)
	}

	k.cmd = exec.Command("keepalived",
		"--dont-fork",
		"--log-console",
		"--release-vips",
		"--log-detail")

	k.cmd.Stdout = os.Stdout
	k.cmd.Stderr = os.Stderr

	k.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	k.started = true

	if err := k.cmd.Run(); err != nil {
		glog.Fatalf("Error starting keepalived: %v", err)
	}
}

// Reload sends SIGHUP to keepalived to reload the configuration.
func (k *keepalived) Reload() error {
	glog.Info("Waiting for keepalived to start")
	for !k.IsRunning() {
		time.Sleep(time.Second)
	}

	k.Cleanup()
	glog.Info("reloading keepalived")
	err := syscall.Kill(k.cmd.Process.Pid, syscall.SIGHUP)
	if err != nil {
		return fmt.Errorf("error reloading keepalived: %v", err)
	}

	return nil
}

// Whether keepalived process is currently running
func (k *keepalived) IsRunning() bool {
	if !k.started {
		glog.Error("keepalived not started")
		return false
	}

	if _, err := os.Stat(keepalivedPid); os.IsNotExist(err) {
		glog.Error("Missing keepalived.pid")
		return false
	}

	return true
}

// Whether keepalived child process is currently running and VIPs are assigned
func (k *keepalived) Healthy() error {
	if !k.IsRunning() {
		return fmt.Errorf("keepalived is not running")
	}

	if _, err := os.Stat(vrrpPid); os.IsNotExist(err) {
		return fmt.Errorf("VRRP child process not running")
	}

	b, err := ioutil.ReadFile(keepalivedState)
	if err != nil {
		return err
	}

	master := false
	state := strings.TrimSpace(string(b))
	if strings.Contains(state, "MASTER") {
		master = true
	}

	var out bytes.Buffer
	cmd := exec.Command("ip", "-brief", "address", "show", k.iface, "up")
	cmd.Stderr = os.Stderr
	cmd.Stdout = &out
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	err = cmd.Run()
	if err != nil {
		return err
	}

	ips := out.String()
	glog.V(3).Infof("Status of %s interface: %s", state, ips)

	for _, vip := range k.vips {
		containsVip := strings.Contains(ips, fmt.Sprintf(" %s/32 ", vip))

		if master && !containsVip {
			return fmt.Errorf("Missing VIP %s on %s", vip, state)
		} else if !master && containsVip {
			return fmt.Errorf("%s should not contain VIP %s", state, vip)
		}
	}

	// All checks successful
	return nil
}

func (k *keepalived) Cleanup() {
	glog.Infof("Cleanup: %s", k.vips)
	for _, vip := range k.vips {
		k.removeVIP(vip)
	}

	err := k.ipt.FlushChain(iptables.TableFilter, iptables.Chain(iptablesChain))
	if err != nil {
		glog.V(2).Infof("unexpected error flushing iptables chain %v: %v", err, iptablesChain)
	}
}

// Stop stop keepalived process
func (k *keepalived) Stop() {
	k.Cleanup()

	err := syscall.Kill(k.cmd.Process.Pid, syscall.SIGTERM)
	if err != nil {
		glog.Errorf("error stopping keepalived: %v", err)
	}
}

func (k *keepalived) removeVIP(vip string) {
	glog.Infof("removing configured VIP %v", vip)
	out, err := k8sexec.New().Command("ip", "addr", "del", vip+"/32", "dev", k.iface).CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Error removing VIP %s: %v\n%s", vip, err, out)
	}
}

func (k *keepalived) loadTemplates() error {
	tmpl, err := template.ParseFiles(keepalivedTmpl)
	if err != nil {
		return err
	}
	k.keepalivedTmpl = tmpl

	tmpl, err = template.ParseFiles(haproxyTmpl)
	if err != nil {
		return err
	}
	k.haproxyTmpl = tmpl

	return nil
}
