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
	docker build -t metalstack/csi-lvm-provisioner:${DOCKER_TAG} . -f cmd/provisioner/Dockerfile
	docker build -t metalstack/csi-lvm-controller:${DOCKER_TAG} . -f cmd/controller/Dockerfile

.PHONY: dockerpush
dockerpush:
	docker push metalstack/csi-lvm-controller:${DOCKER_TAG}
	docker push metalstack/csi-lvm-provisioner:${DOCKER_TAG}

.PHONY: clean
clean:
	rm -f bin/*

.PHONY: tests
tests:
	@if minikube status >/dev/null 2>/dev/null; then echo "a minikube is already running. Exiting ..."; exit 1; fi
	@echo "Starting minikube testing setup ... please wait ..."
	@./deploy/start-minikube-on-linux.sh >/dev/null 2>/dev/null
	@kubectl config view --flatten --minify > tests/files/.kubeconfig
	@minikube docker-env > tests/files/.dockerenv
	@sh -c '. ./tests/files/.dockerenv && docker build -t metalstack/csi-lvm-provisioner . -f cmd/provisioner/Dockerfile'
	@sh -c '. ./tests/files/.dockerenv && docker build -t metalstack/csi-lvm-controller . -f cmd/controller/Dockerfile'
	@sh -c '. ./tests/files/.dockerenv && docker build -t csi-lvm-tests tests' >/dev/null
	@sh -c '. ./tests/files/.dockerenv && docker run --rm csi-lvm-tests bats /bats'
	@rm tests/files/.dockerenv
	@rm tests/files/.kubeconfig
	@minikube delete

.PHONY: cijob
cijob:	
	./tests/files/start-minikube-on-github.sh 
	kubectl config view --flatten --minify > tests/files/.kubeconfig
	docker build -t metalstack/csi-lvm-provisioner . -f cmd/provisioner/Dockerfile
	docker build -t metalstack/csi-lvm-controller . -f cmd/controller/Dockerfile
	docker build -t csi-lvm-tests tests > /dev/null
	docker run --rm csi-lvm-tests bats /bats
