package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/google/lvmd/commands"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/kubectl/pkg/scheme"

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var (
	envDirectory     = "CSI_LVM_MOUNTPOINT"
	envLvmType       = "CSI_LVM_TYPE"
	envDevicePattern = "CSI_LVM_DEVICE_PATTERN"
	envNodeName      = "CSI_NODE_NAME"
)

func reviveLVsCmd() *cli.Command {
	return &cli.Command{
		Name: "revivelvs",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  flagVGName,
				Usage: "Required. the name of the volumegroup",
				Value: "csi-lvm",
			},
			&cli.StringFlag{
				Name:    flagDirectory,
				Usage:   "Required. the name of the directory to mount the lv",
				EnvVars: []string{envDirectory},
				Value:   "/tmp/csi-lvm",
			},
			&cli.StringFlag{
				Name:    flagLVMType,
				Usage:   "Required. the lvmType used by health checks",
				EnvVars: []string{envLvmType},
				Value:   "linear",
			},
			&cli.StringSliceFlag{
				Name:    flagDevicesPattern,
				Usage:   "Required. the patterns of the physical volumes to use.",
				EnvVars: []string{envDevicePattern},
			},
			&cli.StringFlag{
				Name:    flagNodeName,
				Usage:   "Required. the name of node.",
				EnvVars: []string{envNodeName},
			},
		},
		Action: func(c *cli.Context) error {
			if err := reviveLVs(c); err != nil {
				klog.Fatalf("Error reviving logical volumes: %v", err)
				return err
			}

			config, err := rest.InClusterConfig()
			if err != nil {
				return errors.Wrap(err, "unable to get client config")
			}

			kubeClient, err := clientset.NewForConfig(config)
			if err != nil {
				return errors.Wrap(err, "unable to get k8s client")
			}
			nodeName := c.String(flagNodeName)
			if nodeName == "" {
				return fmt.Errorf("invalid nodename %s", nodeName)
			}

			// stay alive
			for {
				err := healthCheck(c)
				if err != nil {
					klog.Error("health check failed")
					err = setNodeNotReady(kubeClient, nodeName)
					if err != nil {
						klog.Error("nodeNotReady for %s failed: %s", nodeName, err)
					}
				} else {
					klog.Info("health check succeeded")
				}
				time.Sleep(1 * time.Minute)
				logStatus()
				time.Sleep(1 * time.Minute)
			}
		},
	}
}

// logStatus will log lvs and vgs to make them visible
func logStatus() {
	cmd := exec.Command("vgs")
	out, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("unable to display volume group:%s %v", out, err)
	}
	klog.Infof("vgs output:%s", out)
	cmd = exec.Command("lvs")
	out, err = cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("unable to display logical volume:%s %v", out, err)
	}
	klog.Infof("lvs output:%s", out)
}

// reviveLVs scans for existing volumes which are not mounted correctly
func reviveLVs(c *cli.Context) error {
	klog.Info("starting reviver")
	vgName := c.String(flagVGName)
	if vgName == "" {
		return fmt.Errorf("invalid empty flag %v", flagVGName)
	}
	dirName := c.String(flagDirectory)
	if dirName == "" {
		return fmt.Errorf("invalid empty flag %v", flagDirectory)
	}
	vgexists := vgExists(vgName)
	if !vgexists {
		klog.Infof("volumegroup: %s not found\n", vgName)
		vgactivate(vgName)
		// now check again for existing vg again
		vgexists = vgExists(vgName)
		if !vgexists {
			klog.Infof("volumegroup: %s not found\n", vgName)
			return nil
		}
	}
	cmd := exec.Command("lvchange", "-ay", vgName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		klog.Infof("unable to activate logical volumes:%s %v", out, err)
	}
	lvs, err := commands.ListLV(context.Background(), vgName)
	if err != nil {
		klog.Infof("unable to list existing logicalvolumes:%v", err)
	}
	for _, lv := range lvs {
		klog.Infof("inspect lv:%s\n", lv.Name)
		targetPath := dirName + "/" + lv.Name
		tp, err := os.Lstat(targetPath)
		if err != nil {
			klog.Infof("target %s is missing. Reviving ...\n", targetPath)
			for _, n := range lv.Tags {
				if n == "isBlock=true" {
					_, err := bindMountLV(lv.Name, vgName, dirName)
					if err != nil {
						klog.Errorf("unable to bind mount lv:%s error:%v", lv.Name, err)
					}
				} else if n == "isBlock=false" {
					_, err := mountLV(lv.Name, vgName, dirName)
					if err != nil {
						klog.Errorf("unable to mount lv:%s error:%v", lv.Name, err)
					}
				}
			}
		} else {
			// Check already existing volumes for missing isBlock tags and add tag if missing
			// This is only needed for migrating from previous csi-lvm versions.
			// This must only run once for every volume created with lvm-csi v0.4.x
			// This else block can be removed later.
			klog.Infof("target %s exists. Checking for missing isBlock tags ...\n", targetPath)
			blockTagFound := false
			for _, n := range lv.Tags {
				if n == "isBlock=true" || n == "isBlock=false" {
					blockTagFound = true
				}
			}
			if !blockTagFound {
				blockMode := true
				if tp.Mode().IsDir() {
					blockMode = false
				}
				klog.Infof("volume %s lacks isBlock tags. Readding isBlock=%t\n", targetPath, blockMode)
				_, err := commands.AddTagLV(context.Background(), vgName, lv.Name, []string{"lv.metal-stack.io/csi-lvm", "isBlock=" + strconv.FormatBool(blockMode)})
				if err != nil {
					klog.Errorf("unable to add tag to lv:%s error:%v", lv.Name, err)
				}
			}
		}
	}
	return nil
}

func healthCheck(c *cli.Context) error {
	var lvName = "canary"
	var lvSize = uint64(10 * 1024)

	klog.Info("starting healthcheck")

	vgName := c.String(flagVGName)
	if vgName == "" {
		return fmt.Errorf("invalid empty flag %v", flagVGName)
	}
	dirName := c.String(flagDirectory)
	if dirName == "" {
		return fmt.Errorf("invalid empty flag %v", flagDirectory)
	}
	lvmType := c.String(flagLVMType)
	if lvmType == "" {
		return fmt.Errorf("invalid empty flag %v", flagLVMType)
	}
	devicesPattern := c.StringSlice(flagDevicesPattern)
	if len(devicesPattern) == 0 {
		return fmt.Errorf("invalid empty flag %v", flagDevicesPattern)
	}

	output, err := createVG(vgName, devicesPattern)
	if err != nil {
		return fmt.Errorf("unable to create vg: %v output:%s", err, output)
	}

	out, err := createLVS(context.Background(), vgName, lvName, lvSize, lvmType, false)
	if err != nil {
		return fmt.Errorf("canary volume %s creation failed: %s", out, err)
	}

	output, err = mountLV(lvName, vgName, dirName)
	if err != nil {
		return fmt.Errorf("unable to mount lv: %v output:%s", err, output)
	}
	klog.Infof("mounted lv %s size:%d vg:%s devices:%s created", lvName, lvSize, vgName, devicesPattern)

	output, err = umountLV(lvName, vgName, dirName)
	if err != nil {
		return fmt.Errorf("unable to umount lv: %v output:%s", err, output)
	}

	output, err = commands.RemoveLV(context.Background(), vgName, lvName)
	if err != nil {
		return fmt.Errorf("unable to delete lv: %v output:%s", err, output)
	}
	return err
}

func setNodeNotReady(client *clientset.Clientset, nodeName string) error {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: client.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "csi-lvm", Host: nodeName})
	// record event
	// csi-lvm       37s         Warning   CSILVMNotWorking     node/m01                          csi-lvm cannot create volumes
	recorder.Event(
		&v1.ObjectReference{
			Kind:      "Node",
			Name:      nodeName,
			UID:       types.UID(nodeName),
			Namespace: "csi-lvm",
		},
		v1.EventTypeWarning, "CSILVMNotWorking", "csi-lvm cannot create volumes")
	// record event
	// default       61s         Warning   NodeNotReady         node/m01                          Node m01 status is now: NodeNotReady
	recorder.Event(
		&v1.ObjectReference{
			Kind:      "Node",
			Name:      nodeName,
			UID:       types.UID(nodeName),
			Namespace: "default",
		},
		v1.EventTypeWarning, "NodeNotReady", fmt.Sprintf("Node %s status is now: NodeNotReady", nodeName))

	// set node to Unschedulable
	// NAME   STATUS                        ROLES    AGE   VERSION
	// m01    NotReady,SchedulingDisabled   master   78m   v1.17.3
	spec := v1.NodeSpec{
		Unschedulable: true,
	}
	raw, err := json.Marshal(v1.NodeSpec{Unschedulable: true})
	if err != nil {
		return err
	}
	patch := []byte(fmt.Sprintf(`{"spec":%s}`, raw))
	klog.Infof("spec: %v, raw: %s, patch: %s", spec, raw, patch)
	_, err = client.CoreV1().Nodes().Patch(nodeName, types.MergePatchType, patch)
	if err != nil {
		return err
	}

	// continuously set nodeReady to false
	// prevents other plugins (node-problem-detector or kubelet itself to set node back to ready again)
	for {
		condition := v1.NodeCondition{
			Type:               v1.NodeReady,
			Status:             v1.ConditionFalse,
			Reason:             "CSILVMNotWorking",
			Message:            "csi-lvm cannot create volumes",
			LastTransitionTime: metav1.Now(),
			LastHeartbeatTime:  metav1.Now(),
		}
		raw, err := json.Marshal(&[]v1.NodeCondition{condition})
		if err != nil {
			return err
		}
		klog.Infof("setting node %s to %v", nodeName, condition)
		patch := []byte(fmt.Sprintf(`{"status":{"conditions":%s}}`, raw))
		_, err = client.CoreV1().Nodes().PatchStatus(nodeName, patch)
		if err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
	}
}
