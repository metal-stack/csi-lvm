module github.com/metal-pod/csi-lvm

go 1.13

require (
	github.com/google/lvmd v0.0.0-20190916151813-e6e28ff087f6
	github.com/miekg/dns v1.1.22 // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.2.1 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/urfave/cli v1.22.1
	k8s.io/api v0.0.0-20190620084959-7cf5895f2711
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/klog v0.3.1
	sigs.k8s.io/sig-storage-lib-external-provisioner v4.0.1+incompatible
)
