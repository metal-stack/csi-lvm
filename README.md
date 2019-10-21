# CSI LVM Provisioner

[![Go Report Card](https://goreportcard.com/badge/github.com/metal-pod/csi-lvm)](https://goreportcard.com/report/github.com/metal-pod/csi-lvm)

## Overview

CSI LVM Provisioner provides a way for the Kubernetes users to utilize the local storage in each node. Based on the user configuration, the LVM Provisioner will create `hostPath` based persistent volume on the node automatically. It utilizes the features introduced by Kubernetes [Local Persistent Volume feature](https://kubernetes.io/blog/2018/04/13/local-persistent-volumes-beta/), but make it a simpler solution than the built-in `local` volume feature in Kubernetes. It created a LVM logical volume on the local disks. A grok pattern, which disks to use can be specified. This Provisioner is derived from the [Local Path Provisioner](https://github.com/rancher/local-path-provisioner).

## Compare to Local Path Provisioner

### Pros

Dynamic provisioning the volume using host path.

* Currently the Kubernetes [Local Volume provisioner](https://github.com/kubernetes-incubator/external-storage/tree/master/local-volume) cannot do dynamic provisioning for the host path volumes.
* Support for volume capacity limit.
* Performance speedup if more than one local disk is available because it defaults the created lv´s to stripe across all physical volumes.

## Requirement

Kubernetes v1.12+.

## Deployment

### Installation

In this setup, the directory `/tmp/csi-lvm/<name of the pv>` will be used across all the nodes as the path for provisioning (a.k.a, store the persistent volume data). The provisioner will be installed in `csi-lvm` namespace by default.

```bash
kubectl apply -f https://raw.githubusercontent.com/metal-pod/csi-lvm/master/deploy/controller.yaml
```

After installation, you should see something like the following:

```bash
$ kubectl -n csi-lvm get pod
NAME                                     READY     STATUS    RESTARTS   AGE
csi-lvm-controller-d744ccf98-xfcbk       1/1       Running   0          7m
```

Check and follow the provisioner log using:

```bash
$ kubectl -n csi-lvm logs -f csi-lvm-controller-d744ccf98-xfcbk
I1021 14:09:31.108535       1 main.go:132] Provisioner started
I1021 14:09:31.108830       1 leaderelection.go:235] attempting to acquire leader lease  csi-lvm/metal-pod.io-csi-lvm...
I1021 14:09:31.121305       1 leaderelection.go:245] successfully acquired lease csi-lvm/metal-pod.io-csi-lvm
I1021 14:09:31.124339       1 controller.go:770] Starting provisioner controller metal-pod.io/csi-lvm_csi-lvm-controller-7f94749d78-t5nh8_17d2f7ef-1375-4e36-aa71-82e237430881!
I1021 14:09:31.126248       1 event.go:258] Event(v1.ObjectReference{Kind:"Endpoints", Namespace:"csi-lvm", Name:"metal-pod.io-csi-lvm", UID:"04da008c-36ec-4966-a4f6-c2028e69cdd5", APIVersion:"v1", ResourceVersion:"589", FieldPath:""}): type: 'Normal' reason: 'LeaderElection' csi-lvm-controller-7f94749d78-t5nh8_17d2f7ef-1375-4e36-aa71-82e237430881 became leader
I1021 14:09:31.225917       1 controller.go:819] Started provisioner controller metal-pod.io/csi-lvm_csi-lvm-controller-7f94749d78-t5nh8_17d2f7ef-1375-4e36-aa71-82e237430881!
```

## Usage

Create a `hostPath` backed Persistent Volume and a pod uses it:

```bash
kubectl create -f https://raw.githubusercontent.com/metal-pod/csi-lvm/master/example/pvc.yaml
kubectl create -f https://raw.githubusercontent.com/metal-pod/csi-lvm/master/example/pod.yaml
```

You should see the PV has been created:

```bash
$ kubectl get pv
NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS    CLAIM                    STORAGECLASS   REASON    AGE
pvc-bc3117d9-c6d3-11e8-b36d-7a42907dda78   50Mi       RWO            Delete           Bound     default/lvm-pvc          csi-lvm                  4s
```

The PVC has been bound:

```bash
$ kubectl get pvc
NAME             STATUS    VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
lvm-pvc          Bound     pvc-bc3117d9-c6d3-11e8-b36d-7a42907dda78   50Mi       RWO            csi-lvm        16s
```

And the Pod started running:

```bash
$ kubectl get pod
NAME          READY     STATUS    RESTARTS   AGE
volume-test   1/1       Running   0          3s
```

Write something into the pod

```bash
kubectl exec volume-test -- sh -c "echo lvm-test > /data/test"
```

Now delete the pod using

```bash
kubectl delete -f https://raw.githubusercontent.com/metal-pod/csi-lvm/master/example/pod.yaml
```

After confirm that the pod is gone, recreated the pod using

```bash
kubectl create -f https://raw.githubusercontent.com/metal-pod/csi-lvm/master/example/pod.yaml
```

Check the volume content:

```bash
$ kubectl exec volume-test cat /data/test
lvm-test
```

Delete the pod and pvc

```bash
kubectl delete -f https://raw.githubusercontent.com/metal-pod/csi-lvm/master/example/pvc.yaml
kubectl delete -f https://raw.githubusercontent.com/metal-pod/csi-lvm/master/example/pod.yaml
```

The volume content stored on the node will be automatically cleaned up. You can check the log of `csi-lvm-controller-xxx` for details.

Now you've verified that the provisioner works as expected.

## Configuration

The configuration of the csi-lvm-controller is done via Environment variables:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: csi-lvm-controller
  namespace: csi-lvm
spec:
  replicas: 1
  selector:
    matchLabels:
      app: csi-lvm-controller
  template:
    metadata:
      labels:
        app: csi-lvm-controller
    spec:
      serviceAccountName: csi-lvm-controller
      containers:
      - name: csi-lvm-controller
        image: metalpod/csi-lvm-controller
        imagePullPolicy: IfNotPresent
        command:
        - /csi-lvm-controller
        args:
        - start
        env:
        - name: CSI_LVM_PROVISIONER_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: CSI_LVM_PROVISIONER_IMAGE
          value: "metalpod/csi-lvm-provisioner"
        - name: CSI_LVM_DEVICE_PATTERN
          value: "/dev/loop[0,1]"
```

### Definition

`CSI_LVM_DEVICE_PATTERN` is a grok pattern to specify which block devices to use for lvm devices on the node. This can be for example `/dev/sd[bcde]` if you want to use only /dev/sdb - /dev/sde.

### PVC Striped

By default the pvc will be a stripe across all found block devices specified by the above grok pattern. The means that all blocks written are spread across 4 devices in chunks. This gives ~4 times the read/write performance for the volume, but also a 4 times higher risk of dataloss in case a single disk fails. If you do not want higher performance but more safety, you can disable that with:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: lvm-pvc-not-striped
  namespace: default
  annotations:
    "striped.metal-pod.io/csi-lvm": "false"
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: csi-lvm
  resources:
    requests:
      storage: 50Mi
```

## Uninstall

Before uninstallation, make sure the PVs created by the provisioner have already been deleted. Use `kubectl get pv` and make sure no PV with StorageClass `csi-lvm`.

To uninstall, execute:

```bash
kubectl delete -f https://raw.githubusercontent.com/metal-pod/csi-lvm/master/deploy/controller.yaml
```