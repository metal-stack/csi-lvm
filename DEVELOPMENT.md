# Local Development

- First start minikube with enough memory.

```bash
minikube start --memory 4g
```

- create 2 loop devices for csi-lvm usage

```bash
minikube ssh 'for i in 0 1; do fallocate -l 1G loop${i} ; sudo losetup -f loop${i}; sudo losetup -a ; done'
```

- set docker environment to point to minikube

```bash
eval $(minikube docker-env)
```

- build docker images of controller and provisioner

```bash
make dockerimages
```

- deploy the controller and start logging in the background

```bash
k apply -f deploy/controller.yaml
stern -n csi-lvm ".*" &
```

- deploy the pvc´s

```bash
k apply -f example/pvc.yaml
```

- start using the pvc in different pod scenarios, see example pod-*.yaml
