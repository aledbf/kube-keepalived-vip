FROM quay.io/kubernetes-ingress-controller/debian-base-amd64:0.1 AS keepalived-builder

RUN apt-get -y update && apt-get -y dist-upgrade \
    && clean-install \
        curl \
        clang \
        libssl-dev \
        libnl-3-dev \
        libnl-route-3-dev \
        libnl-genl-3-dev \
        iptables-dev \
        libnfnetlink-dev \
        libiptcdata0-dev \
        libipset-dev \
        make \
        libsnmp-dev \
        automake \
        ca-certificates

ARG KEEPALIVED_VERSION=2.0.20

RUN cd /tmp \
    && curl -sSL https://github.com/acassen/keepalived/archive/v$KEEPALIVED_VERSION.tar.gz | tar -xvz

RUN cd /tmp/keepalived-$KEEPALIVED_VERSION \
    && aclocal \
    && autoheader \
    && automake --add-missing \
    && autoreconf \
    && export CC=clang \
    && ./configure \
        --prefix=/usr \
        --sysconfdir=/etc \
        --enable-bfd \
        --enable-snmp \
        --enable-sha1 \
        --enable-json \
        --enable-optimise \
        --enable-timer-check \
        --enable-netlink-timers \
        --enable-network-timestamp \
        --disable-dynamic-linking \
    && make -j$(nproc) \
    && make DESTDIR=/built install

FROM golang:1.14 AS controller-builder

ENV PKG=github.com/aledbf/kube-keepalived-vip

COPY . src/${PKG}

RUN cd src/${PKG} \
    && mkdir /built \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -a -o /built/kube-keepalived-vip ${PKG}/pkg/cmd \
    && cp -r rootfs/* /built

FROM quay.io/kubernetes-ingress-controller/debian-base-amd64:0.1

RUN clean-install \
        libssl1.1 \
        libnl-3-200 \
        libnl-route-3-200 \
        libnl-genl-3-200 \
        iptables \
        libxtables12 \
        libnfnetlink0 \
        libiptcdata0 \
        libipset11 \
        libipset-dev \
        libsnmp30 \
        kmod \
        ca-certificates \
        iproute2 \
        curl \
        ipvsadm \
        ipset \
        bash \
        jq \
        dumb-init

COPY --from=keepalived-builder /built /

COPY --from=controller-builder /built /

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/kube-keepalived-vip"]