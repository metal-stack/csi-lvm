GO111MODULE := on

.PHONY: all
all:
	go build -tags netgo -ldflags "-linkmode external -extldflags '-static -s -w'" -o bin/csi-lvm-provisioner
	strip bin/csi-lvm-provisioner

.PHONY: dockerimage
dockerimage:
	docker build -t metalpod/csi-lvm:latest .

.PHONY: dockerpush
dockerpush:
	docker push metalpod/csi-lvm:latest
