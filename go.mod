module github.com/metal-stack/csi-lvm

go 1.13

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/google/lvmd v0.0.0-20190916151813-e6e28ff087f6
	github.com/miekg/dns v1.1.28 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.1 // indirect
	github.com/urfave/cli/v2 v2.2.0
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a // indirect
	golang.org/x/sys v0.0.0-20200302150141-5c8b2ff67527 // indirect
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v0.17.4
	k8s.io/klog v1.0.0
	sigs.k8s.io/sig-storage-lib-external-provisioner v4.1.0+incompatible
)
