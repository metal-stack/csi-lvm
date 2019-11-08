GO111MODULE := on
DOCKER_TAG := $(or ${GITHUB_TAG_NAME}, latest)

all: provisioner controller

.PHONY: provisioner
provisioner:
	go build -tags netgo -o bin/csi-lvm-provisioner cmd/provisioner/*.go
	strip bin/csi-lvm-provisioner

.PHONY: controller
controller:
	go build -tags netgo -o bin/csi-lvm-controller cmd/controller/*.go
	strip bin/csi-lvm-controller

.PHONY: dockerimages
dockerimages:
	docker build -t metalpod/csi-lvm-provisioner:${DOCKER_TAG} . -f cmd/provisioner/Dockerfile
	docker build -t metalpod/csi-lvm-controller:${DOCKER_TAG} . -f cmd/controller/Dockerfile

.PHONY: dockerpush
dockerpush:
	docker push metalpod/csi-lvm-controller:${DOCKER_TAG}
	docker push metalpod/csi-lvm-provisioner:${DOCKER_TAG}

.PHONY: clean
clean:
	rm -f bin/*
