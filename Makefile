GO111MODULE := on

.PHONY: all
all:
	go build -tags netgo -o bin/csi-lvm-provisioner
	strip bin/csi-lvm-provisioner

.PHONY: dockerimage
dockerimage:
	docker build -t metalpod/csi-lvm:latest .

.PHONY: dockerpush
dockerpush:
	docker push metalpod/csi-lvm:latest
