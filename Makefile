GO111MODULE := on


.PHONY: provisioner
provisioner:
	go build -tags netgo -o bin/csi-lvm-provisioner cmd/provisioner/*.go
	strip bin/csi-lvm-provisioner

.PHONY: controller
controller:
	go build -tags netgo -o bin/csi-lvm-controller
	strip bin/csi-lvm-controller

.PHONY: dockerimages
dockerimages:
	docker build -t metalpod/csi-lvm-controller:latest .
	docker build -t metalpod/csi-lvm-provisioner:latest . -f cmd/provisioner/Dockerfile

.PHONY: dockerpush
dockerpush:
	docker push metalpod/csi-lvm-controller:latest
	docker push metalpod/csi-lvm-provisioner:latest

.PHONY: clean
clean:
	rm -f bin/*
