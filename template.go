package main

import (
	"os"
	"text/template"
)

func writeBGPCfg(myIP string, neighbors []string) (err error) {
	w, err := os.Create("/etc/exabgp/exabgp.conf")
	if err != nil {
		return err
	}

	t, err := template.ParseFiles("/exabgp.templ")
	if err != nil {
		return err
	}

	conf := make(map[string]interface{})
	conf["myIP"] = myIP
	conf["neighbors"] = neighbors
	return t.Execute(w, conf)
}

func writeHealthcheck(vips []string) (err error) {
	w, err := os.Create("/etc/exabgp/healthcheck.sh")
	if err != nil {
		return err
	}

	t, err := template.ParseFiles("/healthcheck.templ")
	if err != nil {
		return err
	}

	conf := make(map[string]interface{})
	conf["vips"] = vips
	return t.Execute(w, conf)
}
