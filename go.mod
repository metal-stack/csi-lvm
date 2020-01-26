module github.com/metal-pod/csi-lvm

go 1.13

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/google/lvmd v0.0.0-20190916151813-e6e28ff087f6
	github.com/miekg/dns v1.1.27 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0 // indirect
	github.com/urfave/cli v1.22.2
	golang.org/x/crypto v0.0.0-20200117160349-530e935923ad // indirect
	golang.org/x/net v0.0.0-20200114155413-6afb5195e5aa // indirect
	golang.org/x/sys v0.0.0-20200124204421-9fbb57f87de9 // indirect
	k8s.io/api v0.16.6
	k8s.io/apimachinery v0.16.6
	k8s.io/client-go v0.16.6
	k8s.io/klog v1.0.0
	sigs.k8s.io/sig-storage-lib-external-provisioner v4.0.1+incompatible
)
