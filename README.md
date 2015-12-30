# kube-keepalived-vip
Kubernetes Virtual IP address/es using keepalived

AKA "how to set up virtual IP addresses in kubernetes using [ipvs](IPVS - The Linux Virtual Server Project)".

## Disclaimer:
- This is a **work in progress**.

## Overview

There are 2 ways to expose a service in the current kubernetes service model:

- Create a cloud load balancer.
- Allocate a port (the same port) on every node in your cluster and proxy traffic through that port to the endpoints.

This just works. What's the issue then? 

The issue is that it does not provide High Availability because beforehand is required to know the IP addresss of the node where is running and in case of a failure the pod can be be moved to a different node. Here is where ipvs could help. 
The idea is to define an IP address per service to expose it outside the Kubernetes cluster and use vrrp to announce this "mapping" in the local network.
With 2 or more instance of the pod running in the cluster is possible to provide high availabity using a single IP address.

## Configuration

To expose a service add the annotation `k8s.io/virtual-ip` in the service with the IP address to be use. This IP must be routable inside the LAN and must be available.
By default the IP address of the service is used to route the traffic.



## Related projects

- https://github.com/kobolog/gorb
- https://github.com/qmsk/clusterf
- https://github.com/osrg/gobgp
