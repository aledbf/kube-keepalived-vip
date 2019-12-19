# kube-keepalived-vip

Kubernetes Virtual IP addresses using [keepalived](http://www.keepalived.org).

The chart supports both the normal mode of operation and PROXY protocol support via HAProxy.

## Prerequisites

- Kubernetes 1.9+
- [Helm](https://helm.sh)

Make sure you have Helm [installed](https://helm.sh/docs/using_helm/#installing-helm) and
[deployed](https://helm.sh/docs/using_helm/#installing-tiller) to your cluster. Then add
the chart repository to Helm:

```bash
$ helm repo add kube-keepalived-vip https://aledbf.github.io/kube-keepalived-vip/
```

## Install/Upgrade

To install or upgrade the chart with the default [configuration](#Configuration) and the release name `my-release`:

```bash
$ helm repo update
$ helm upgrade --install my-release kube-keepalived-vip/kube-keepalived-vip
```

## Uninstall

Removes all the Kubernetes components associated with the chart and deletes the release:

```bash
$ helm delete my-release
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `fullnameOverride` | The full name to use for resources | `` |
| `haproxy.enabled` | Enables the PROXY protocol feature | `false` |
| `haproxy.image.pullPolicy` | HAProxy image pull policy | `IfNotPresent` |
| `haproxy.image.repository` | HAProxy image name | `aledbf/haproxy-self-reload` |
| `haproxy.image.tag` | HAProxy image version | See values.yaml |
| `haproxy.name` | HAProxy container name | `haproxy` |
| `haproxy.resources` | Resource allocations for the HAProxy container | `{}` |
| `httpPort` | Port used for health checks | `8080` |
| `keepalived.config` | Content of the configuration supplied to kube-keepalive-vip | `{}` |
| `keepalived.image.pullPolicy` | Keepalived image pull policy | `IfNotPresent` |
| `keepalived.image.repository` | Keepalived image name | `aledbf/kube-keepalived-vip` |
| `keepalived.image.tag` | Keepalived image version | See values.yaml |
| `keepalived.name` | Keepalived container name | `keepalived` |
| `keepalived.resources` | Resource allocations for the keepalived container | `{}` |
| `keepalived.useUnicast` | Unicast uses the IP of the nodes instead of multicast. This is useful if running in cloud providers (like AWS). | `false` |
| `keepalived.vrid` | VRRP virtual router ID, must be unique on a particular network segment (0-255) | `179` |
| `nameOverride` | The partial name to use for resources | `` |
| `podAnnotations` | Pod annotations | `{}` |
| `podLabels` | Pod labels | `{}` |
| `nodeSelector` | Node selector | `{}` |
| `rbac.create` | Whether to create RBAC resources | `true` |
| `rbac.serviceAccountName` | Only required when `rbac.create` is false | `default` |
| `revisionHistoryLimit` | DaemonSet revision history | `10` |
| `tolerations` | Tolerations | `[]` |
| `updateStrategy` | DaemonSet update strategy | `{}` |
