all: push

# 0.0 shouldn't clobber any release builds
TAG = 0.19
PREFIX = aledbf/kube-keepalived-vip
BUILD_IMAGE = build-keepalived
PKG = github.com/aledbf/kube-keepalived-vip

controller: clean
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-s -w' \
	-o rootfs/kube-keepalived-vip \
	${PKG}/pkg/cmd

container: controller keepalived
	docker build -t $(PREFIX):$(TAG) rootfs

keepalived:
	docker build -t $(BUILD_IMAGE):$(TAG) build
	docker create --name $(BUILD_IMAGE) $(BUILD_IMAGE):$(TAG) true
	# docker cp semantics changed between 1.7 and 1.8, so we cp the file to cwd and rename it.
	docker cp $(BUILD_IMAGE):/keepalived.tar.gz rootfs
	docker rm -f $(BUILD_IMAGE)

push: container
	docker push $(PREFIX):$(TAG)

clean:
	rm -f kube-keepalived-vip
