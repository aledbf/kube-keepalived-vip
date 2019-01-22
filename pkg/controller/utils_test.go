/*
Copyright 2016 The Kubernetes Authors.

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
	"errors"
	"fmt"
	"testing"
)

func TestParseNsSvcLVS(t *testing.T) {
	testcases := map[string]struct {
		Input         string
		Namespace     string
		Service       string
		ForwardMethod string
		ErrorExpected bool
	}{
		"just service name":      {"echoheaders", "", "", "", true},
		"missing namespace":      {"echoheaders:NAT", "", "", "", true},
		"default forward method": {"default/echoheaders", "default", "echoheaders", "NAT", false},
		"with forward method":    {"default/echoheaders:NAT", "default", "echoheaders", "NAT", false},
		"DR as forward method":   {"default/echoheaders:DR", "default", "echoheaders", "DR", false},
		"invalid forward method": {"default/echoheaders:AJAX", "", "", "", true},
	}

	for k, tc := range testcases {
		ns, svc, lvs, err := parseNsSvcLVS(tc.Input)

		if tc.ErrorExpected && err == nil {
			t.Errorf("%s: expected an error but valid information returned: %v ", k, tc.Input)
		}

		if tc.Namespace != ns {
			t.Errorf("%s: expected %v but returned %v - input %v", k, tc.Namespace, ns, tc.Input)
		}

		if tc.Service != svc {
			t.Errorf("%s: expected %v but returned %v - input %v", k, tc.Service, svc, tc.Input)
		}

		if tc.ForwardMethod != lvs {
			t.Errorf("%s: expected %v but returned %v - input %v", k, tc.ForwardMethod, lvs, tc.Input)
		}
	}
}

func TestParseAddress(t *testing.T) {
	table := []struct {
		address           string
		wantIp, wantIface string
		wantErr           error
	}{
		{
			"1-192.168.0.1@eth0",
			"192.168.0.1",
			"eth0",
			nil,
		},
		{
			"1-2001:db8:85a3::8a2e:370:7334@eth0",
			"2001:db8:85a3::8a2e:370:7334",
			"eth0",
			nil,
		},
		{
			"192.168.0.1@eth0",
			"192.168.0.1",
			"eth0",
			nil,
		},
		{
			"192.168.0.1",
			"192.168.0.1",
			"",
			nil,
		},
		{
			"eth0",
			"",
			"",
			errors.New(`invalid address tring: "eth0"`),
		},
	}

	for i, v := range table {
		ip, iface, err := parseAddress(v.address)
		if ip != v.wantIp {
			t.Errorf("Unexpected ip in %d: want: %q, got: %q", i, v.wantIp, ip)
		}
		if iface != v.wantIface {
			t.Errorf("Unexpected interface name in %d: want: %q, got: %q", i, v.wantIface, iface)
		}
		if fmt.Sprintf("%s", v.wantErr) != fmt.Sprintf("%s", err) {
			t.Errorf("Unexpected error in %d: want: %q, got: %q", i, v.wantErr, err)
		}
	}
}
