module github.com/metal-stack/csi-lvm

go 1.14

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/google/lvmd v0.0.0-20190916151813-e6e28ff087f6
	github.com/miekg/dns v1.1.29 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.6.0 // indirect
	github.com/urfave/cli/v2 v2.2.0
	golang.org/x/net v0.0.0-20200506145744-7e3656a0809f // indirect
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v0.17.4
	k8s.io/klog v1.0.0
	sigs.k8s.io/sig-storage-lib-external-provisioner/v5 v5.0.0
)
