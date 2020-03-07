all: push

# 0.0 shouldn't clobber any release builds
TAG = 0.35
HAPROXY_TAG = 0.1
# Helm uses SemVer2 versioning
CHART_VERSION = 1.0.0
PREFIX = aledbf/kube-keepalived-vip
PKG = github.com/aledbf/kube-keepalived-vip

GO_LIST_FILES=$(shell go list ${PKG}/... | grep -v vendor)

container:
	# Use docker buildkit if available
	export DOCKER_BUILDKIT=1
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

.PHONY: chart
chart: chart/kube-keepalived-vip-$(CHART_VERSION).tgz

.PHONY: chart-subst
chart-subst: chart/kube-keepalived-vip/Chart.yaml.tmpl chart/kube-keepalived-vip/values.yaml.tmpl
	for file in Chart.yaml values.yaml; do cp -f "chart/kube-keepalived-vip/$$file.tmpl" "chart/kube-keepalived-vip/$$file"; done
	sed -i'.bak' -e 's|%%TAG%%|$(TAG)|g' -e 's|%%HAPROXY_TAG%%|$(HAPROXY_TAG)|g' chart/kube-keepalived-vip/values.yaml
	sed -i'.bak' -e 's|%%CHART_VERSION%%|$(CHART_VERSION)|g' chart/kube-keepalived-vip/Chart.yaml
	rm -f chart/kube-keepalived-vip/*.bak

# Requires helm
chart/kube-keepalived-vip-$(CHART_VERSION).tgz: chart-subst $(shell which helm) $(shell find chart/kube-keepalived-vip -type f)
	helm lint --strict chart/kube-keepalived-vip
	helm package --version '$(CHART_VERSION)' -d chart chart/kube-keepalived-vip

clean:
	rm -f kube-keepalived-vip

.PHONY: fmt
fmt:
	@go list -f '{{if len .TestGoFiles}}"gofmt -s -l {{.Dir}}"{{end}}' ${GO_LIST_FILES} | xargs -L 1 sh -c

.PHONY: lint
lint:
	@go list -f '{{if len .TestGoFiles}}"golint -min_confidence=0.85 {{.Dir}}/..."{{end}}' ${GO_LIST_FILES} | xargs -L 1 sh -c

.PHONY: test
test:
	@go test -v -race -tags "$(BUILDTAGS) cgo" ${GO_LIST_FILES}

.PHONY: cover
cover:
	@go list -f '{{if len .TestGoFiles}}"go test -coverprofile={{.Dir}}/.coverprofile {{.ImportPath}}"{{end}}' ${GO_LIST_FILES} | xargs -L 1 sh -c
	gover
	goveralls -coverprofile=gover.coverprofile -service travis-ci

.PHONY: vet
vet:
	@go vet ${GO_LIST_FILES}

.PHONY: dep-ensure
dep-ensure:
	GO111MODULE=on go mod tidy -v
	find vendor -name '*_test.go' -delete
	GO111MODULE=on go mod vendor

