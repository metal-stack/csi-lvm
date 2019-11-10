package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"github.com/urfave/cli"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	pvController "sigs.k8s.io/sig-storage-lib-external-provisioner/controller"
)

var (
	flagProvisionerName     = "provisioner-name"
	envProvisionerName      = "PROVISIONER_NAME"
	defaultProvisionerName  = "metal-pod.io/csi-lvm"
	flagNamespace           = "namespace"
	envNamespace            = "CSI_LVM_PROVISIONER_NAMESPACE"
	defaultNamespace        = "csi-lvm"
	flagProvisionerImage    = "provisioner-image"
	envProvisionerImage     = "CSI_LVM_PROVISIONER_IMAGE"
	defaultProvisionerImage = "metalpod/csi-lvm-provisioner"
	flagDevicePattern       = "device-pattern"
	envDevicePattern        = "CSI_LVM_DEVICE_PATTERN"
	flagDefaultLVMType      = "default-lvm-type"
	envDefaultLVMType       = "CSI_LVM_DEFAULT_LVM_TYPE"
	flagMountPoint          = "mountpoint"
	envMountPoint           = "CSI_LVM_MOUNTPOINT"
)

func cmdNotFound(c *cli.Context, command string) {
	panic(fmt.Errorf("Unrecognized command: %s", command))
}

func onUsageError(c *cli.Context, err error, isSubcommand bool) error {
	panic(fmt.Errorf("Usage error, please check your command"))
}

func registerShutdownChannel(done chan struct{}) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		klog.Infof("Receive %v to exit", sig)
		close(done)
	}()
}

func startCmd() cli.Command {
	return cli.Command{
		Name: "start",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   flagProvisionerName,
				Usage:  "Required. Specify Provisioner name.",
				EnvVar: envProvisionerName,
				Value:  defaultProvisionerName,
			},
			cli.StringFlag{
				Name:   flagNamespace,
				Usage:  "Required. The namespace that Provisioner is running in",
				EnvVar: envNamespace,
				Value:  defaultNamespace,
			},
			cli.StringFlag{
				Name:   flagProvisionerImage,
				Usage:  "Required. The provisioner image used for create/delete lvm volumes on the host",
				EnvVar: envProvisionerImage,
				Value:  defaultProvisionerImage,
			},
			cli.StringFlag{
				Name:   flagDevicePattern,
				Usage:  "Required. The pattern of the disk devices on the node to use",
				EnvVar: envDevicePattern,
			},
			cli.StringFlag{
				Name:   flagDefaultLVMType,
				Usage:  "Optional. the default lvm type to use, must be one of linear|striped|mirror",
				EnvVar: envDefaultLVMType,
				Value:  mirrorType,
			},
			cli.StringFlag{
				Name:   flagMountPoint,
				Usage:  "Optional. the mountpoint on the node where the volumes get mounted",
				EnvVar: envMountPoint,
				Value:  "/tmp/csi-lvm",
			},
		},
		Action: func(c *cli.Context) {
			if err := startDaemon(c); err != nil {
				klog.Fatalf("Error starting daemon: %v", err)
			}
		},
	}
}

func startDaemon(c *cli.Context) error {
	stopCh := make(chan struct{})
	registerShutdownChannel(stopCh)

	config, err := rest.InClusterConfig()
	if err != nil {
		return errors.Wrap(err, "unable to get client config")
	}

	kubeClient, err := clientset.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "unable to get k8s client")
	}

	serverVersion, err := kubeClient.Discovery().ServerVersion()
	if err != nil {
		return errors.Wrap(err, "Cannot start Provisioner: failed to get Kubernetes server version")
	}

	provisionerName := c.String(flagProvisionerName)
	if provisionerName == "" {
		return fmt.Errorf("invalid empty flag %v", flagProvisionerName)
	}
	namespace := c.String(flagNamespace)
	if namespace == "" {
		return fmt.Errorf("invalid empty flag %v", flagNamespace)
	}
	provisionerImage := c.String(flagProvisionerImage)
	if provisionerImage == "" {
		return fmt.Errorf("invalid empty flag %v", flagProvisionerImage)
	}
	devicePattern := c.String(flagDevicePattern)
	if devicePattern == "" {
		return fmt.Errorf("invalid empty flag %v", flagDevicePattern)
	}

	defaultLVMType := c.String(flagDefaultLVMType)
	if defaultLVMType == "" {
		return fmt.Errorf("invalid empty flag %v", flagDefaultLVMType)
	}
	mountPoint := c.String(flagMountPoint)
	if mountPoint == "" {
		return fmt.Errorf("invalid empty flag %v", flagMountPoint)
	}

	provisioner := NewLVMProvisioner(kubeClient, namespace, mountPoint, devicePattern, provisionerImage, defaultLVMType)
	if err != nil {
		return err
	}
	pc := pvController.NewProvisionController(
		kubeClient,
		provisionerName,
		provisioner,
		serverVersion.GitVersion,
	)
	klog.Info("Provisioner started")
	pc.Run(stopCh)
	klog.Info("Provisioner stopped")
	return nil
}

func main() {
	a := cli.NewApp()
	a.Usage = "LVM Provisioner"
	a.Commands = []cli.Command{
		startCmd(),
	}
	a.CommandNotFound = cmdNotFound
	a.OnUsageError = onUsageError

	if err := a.Run(os.Args); err != nil {
		klog.Fatalf("Critical error: %v", err)
	}
}
