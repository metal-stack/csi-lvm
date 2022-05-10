package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	pvController "sigs.k8s.io/sig-storage-lib-external-provisioner/v8/controller"
)

var (
	flagProvisionerName          = "provisioner-name"
	envProvisionerName           = "PROVISIONER_NAME"
	defaultProvisionerName       = "metal-stack.io/csi-lvm"
	flagNamespace                = "namespace"
	envNamespace                 = "CSI_LVM_PROVISIONER_NAMESPACE"
	defaultNamespace             = "csi-lvm"
	flagVgName                   = "vgname"
	envVgName                    = "CSI_LVM_VG_NAME"
	flagProvisionerImage         = "provisioner-image"
	envProvisionerImage          = "CSI_LVM_PROVISIONER_IMAGE"
	defaultProvisionerImage      = "ghcr.io/metal-stack/csi-lvm-provisioner"
	flagDevicePattern            = "device-pattern"
	envDevicePattern             = "CSI_LVM_DEVICE_PATTERN"
	flagDefaultLVMType           = "default-lvm-type"
	envDefaultLVMType            = "CSI_LVM_DEFAULT_LVM_TYPE"
	flagMountPoint               = "mountpoint"
	envMountPoint                = "CSI_LVM_MOUNTPOINT"
	flagProvisionerPodPullPolicy = "pull-policy"
	envProvisionerPodPullPolicy  = "CSI_LVM_PULL_POLICY"
)

func cmdNotFound(c *cli.Context, command string) {
	panic(fmt.Errorf("unrecognized command: %s", command))
}

func onUsageError(c *cli.Context, err error, isSubcommand bool) error {
	panic(fmt.Errorf("usage error, please check your command"))
}

func startCmd() *cli.Command {
	return &cli.Command{
		Name: "start",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    flagProvisionerName,
				Usage:   "Required. Specify Provisioner name.",
				EnvVars: []string{envProvisionerName},
				Value:   defaultProvisionerName,
			},
			&cli.StringFlag{
				Name:    flagNamespace,
				Usage:   "Required. The namespace that Provisioner is running in",
				EnvVars: []string{envNamespace},
				Value:   defaultNamespace,
			},
			&cli.StringFlag{
				Name:    flagVgName,
				Usage:   "Required. LVM volume group name",
				EnvVars: []string{envVgName},
				Value:   "csi-lvm",
			},
			&cli.StringFlag{
				Name:    flagProvisionerImage,
				Usage:   "Required. The provisioner image used for create/delete lvm volumes on the host",
				EnvVars: []string{envProvisionerImage},
				Value:   defaultProvisionerImage,
			},
			&cli.StringFlag{
				Name:    flagDevicePattern,
				Usage:   "Required. The pattern of the disk devices on the node to use",
				EnvVars: []string{envDevicePattern},
			},
			&cli.StringFlag{
				Name:    flagDefaultLVMType,
				Usage:   "Optional. the default lvm type to use, must be one of linear|striped|mirror",
				EnvVars: []string{envDefaultLVMType},
				Value:   mirrorType,
			},
			&cli.StringFlag{
				Name:    flagMountPoint,
				Usage:   "Optional. the mountpoint on the node where the volumes get mounted",
				EnvVars: []string{envMountPoint},
				Value:   "/tmp/csi-lvm",
			},
			&cli.StringFlag{
				Name:    flagProvisionerPodPullPolicy,
				Usage:   "Optional. the pull policy for the provisioner pod, can be Always|IfNotPresent",
				EnvVars: []string{envProvisionerPodPullPolicy},
				Value:   pullAlways,
			},
		},
		Action: func(c *cli.Context) error {
			if err := startDaemon(c); err != nil {
				klog.Fatalf("Error starting daemon: %v", err)
				return err
			}
			return nil
		},
	}
}

func startDaemon(c *cli.Context) error {

	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("unable to get client config %w", err)
	}

	kubeClient, err := clientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("unable to get k8s client %w", err)
	}

	provisionerName := c.String(flagProvisionerName)
	if provisionerName == "" {
		return fmt.Errorf("invalid empty flag %v", flagProvisionerName)
	}
	namespace := c.String(flagNamespace)
	if namespace == "" {
		return fmt.Errorf("invalid empty flag %v", flagNamespace)
	}
	vgName := c.String(flagVgName)
	if vgName == "" {
		return fmt.Errorf("invalid empty flag %v", flagVgName)
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
	pullPolicy := c.String(flagProvisionerPodPullPolicy)
	if pullPolicy == "" {
		return fmt.Errorf("invalid empty flag %v", flagProvisionerPodPullPolicy)
	}

	provisioner := NewLVMProvisioner(kubeClient, namespace, vgName, mountPoint, devicePattern, provisionerImage, defaultLVMType, pullPolicy)

	pc := pvController.NewProvisionController(
		kubeClient,
		provisionerName,
		provisioner,
	)
	klog.Info("Provisioner started")
	pc.Run(context.Background())
	klog.Info("Provisioner stopped")
	return nil
}

func main() {
	a := cli.NewApp()
	a.Usage = "LVM Provisioner"
	a.Commands = []*cli.Command{
		startCmd(),
	}
	a.CommandNotFound = cmdNotFound
	a.OnUsageError = onUsageError

	if err := a.Run(os.Args); err != nil {
		klog.Fatalf("Critical error: %v", err)
	}
}
