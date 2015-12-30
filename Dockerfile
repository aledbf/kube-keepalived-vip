FROM alpine:3.2

COPY keepalived.apk /var/cache/apk/keepalived.apk

RUN apk add -U --repository http://dl-3.alpinelinux.org/alpine/edge/testing/ --allow-untrusted \
  bash curl ipvsadm iproute2 && \
  apk add --allow-untrusted /var/cache/apk/keepalived.apk && \
  rm -rf /var/cache/apk/* && \
  mkdir -p /etc/keepalived

COPY kube-keepalived-vip /kube-keepalived-vip

ENTRYPOINT ["/kube-keepalived-vip"]
