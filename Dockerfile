FROM alpine:3.2

RUN apk add -U --repository http://dl-3.alpinelinux.org/alpine/edge/testing/ \
  --allow-untrusted \
  bash curl ipvsadm iproute2 python-dev && \  
  rm -rf /var/cache/apk/*

RUN curl -sSL https://raw.githubusercontent.com/pypa/pip/7.1.2/contrib/get-pip.py | python -

RUN pip install exabgp

COPY kube-bgp-vip /kube-bgp-vip

COPY bgp/exabgp.env /usr/etc/exabgp/exabgp.env

ENTRYPOINT ["/kube-bgp-vip"]
