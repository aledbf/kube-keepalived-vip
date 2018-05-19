
## 0.27

- Update keepalived to v1.4.4

## 0.26

- Fix keepalived SIGSEGV in k8s 1.9.x


## 0.25

- Update go dependencies
- [X] [#8](https://github.com/aledbf/kube-keepalived-vip/pull/35) Fix keepalived dependencies

## 0.24

- Update keepalived to v1.4.0
- Change docker base image to gcr.io/google-containers/debian-base-amd64:0.3

## 0.23

- Update keepalived to v1.3.9

## 0.22

- Update keepalived to v1.3.7

## 0.21

- Fix template error

## 0.20

- [X] [#8](https://github.com/aledbf/kube-keepalived-vip/pull/8) Fix keepalived config file error when there are no services
- [X] [#9](https://github.com/aledbf/kube-keepalived-vip/pull/9) Add support for travis-ci
- [X] [#10](https://github.com/aledbf/kube-keepalived-vip/pull/10) Add badges
- [X] [#11](https://github.com/aledbf/kube-keepalived-vip/pull/11) Fix badges
- [X] [#17](https://github.com/aledbf/kube-keepalived-vip/pull/17) Remove unnecessary backquote and space
- [X] [#18](https://github.com/aledbf/kube-keepalived-vip/pull/18) VRID flag not supported in keepalived.tmpl
- [X] [#19](https://github.com/aledbf/kube-keepalived-vip/pull/19) Use vrid flag in template


## 0.17
- Cleanup
- Configmap update detection

## 0.16
- Fix DR mode issue

## 0.15
- Update keepalived to 1.3.6
- Update go dependencies
- Migrate to client-go

## 0.10
- Add proxy mode
- Update keepalived to 1.3.2
- Update godeps
- Fix network interface detection issues

## 0.9
- Update godeps
- Update keepalived to 1.2.24

## 0.8
- Update godeps - required by  kubernetes/kubernetes#31396
- Update ubuntu-slim to 0.4

## 0.7
- Update keepalived to 1.2.23
- Use delayed queue
- Avoid sync without a reachable master

## 0.6
- Update keepalived to 1.2.21

## 0.5
- Respect nodeSelector to build the list of peer nodes when --use-unicast is true 
- Add support for UDP services

## 0.4
- Update keepalived to 1.2.20
- Use iptables parameter to not respond on addresses that it does not own
- Replace annotations with ConfigMap to specify services and IP addresses
- Avoid unnecessary reloads if the configuration did not change
- The parameter --password was removed because is not supported in vrrp v3
