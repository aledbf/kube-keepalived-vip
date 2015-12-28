package main

import (
	"os"
	"text/template"
)

const (
	exabgp = `{{ $myIP := .myIP }}
group anycast {
  router-id {{ $myIP }};
  peer-as 65000;
  local-as 65000;

  process periodic-announce {
    run /healthcheck.sh;
  }

  {{ range $i, $neighbor := .neighbors }}
  neighbor {{ $neighbor }} {
    local-address {{ $myIP }};
  }
  {{ end }}
}
`

	healthcheck = `#!/usr/bin/env python
 
from sys import stdout
from time import sleep
 
messages = [{{ range $i, $vip := .vips }}
  'announce route {{ $vip }}/32 next-hop self',
  {{ end }}
]

sleep(5)
 
#Iterate through messages
for message in messages:
    stdout.write( message + '\n')
    stdout.flush()
    sleep(1)
 
#Loop endlessly to allow ExaBGP to continue running
while True:
    sleep(1)
`
)

func writeBGPCfg(myIP string, neighbors []string) error {
	w, err := os.Create("/exabgp.conf")
	if err != nil {
		return err
	}
	defer w.Close()

	t, err := template.New("exabgp").Parse(exabgp)
	if err != nil {
		return err
	}

	conf := make(map[string]interface{})
	conf["myIP"] = myIP
	conf["neighbors"] = neighbors
	return t.Execute(w, conf)
}

func writeHealthcheck(myIP string, vips []string) error {
	w, err := os.Create("/healthcheck.sh")
	if err != nil {
		return err
	}
	defer w.Close()

	t, err := template.New("healthcheck").Parse(healthcheck)
	if err != nil {
		return err
	}

	conf := make(map[string]interface{})
	conf["myIP"] = myIP
	conf["vips"] = vips

	if err = t.Execute(w, conf); err != nil {
		return err
	}

	return w.Chmod(0755)
}
