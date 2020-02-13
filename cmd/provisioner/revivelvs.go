package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/google/lvmd/commands"
	"github.com/urfave/cli/v2"
	"k8s.io/klog"
)

var (
	envDirectory = "CSI_LVM_MOUNTPOINT"
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
		},
		Action: func(c *cli.Context) error {
			if err := reviveLVs(c); err != nil {
				klog.Fatalf("Error reviving logical volumes: %v", err)
				return err
			}
			// stay alive
			for {
				time.Sleep(time.Hour)
			}
		},
	}
}

// reviveLVs scans for existing volumes which are not mounted correctly
func reviveLVs(c *cli.Context) error {
	klog.Infof("starting reviver: %s", c)
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
					bindMountLV(lv.Name, vgName, dirName)
				} else if n == "isBlock=false" {
					mountLV(lv.Name, vgName, dirName)
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
				commands.AddTagLV(context.Background(), vgName, lv.Name, []string{"lv.metal-stack.io/csi-lvm", "isBlock=" + strconv.FormatBool(blockMode)})
			}
		}
	}
	return nil
}
