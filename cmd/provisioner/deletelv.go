package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/google/lvmd/commands"
	"github.com/urfave/cli"
	"k8s.io/klog"
)

func deleteLVCmd() cli.Command {
	return cli.Command{
		Name: "deletelv",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  flagLVName,
				Usage: "Required. Specify lv name.",
			},
			cli.StringFlag{
				Name:  flagVGName,
				Usage: "Required. the name of the volumegroup",
			},
			cli.StringFlag{
				Name:  flagDirectory,
				Usage: "Required. the name of the directory to mount the lv",
			},
			cli.BoolFlag{
				Name:  flagBlockMode,
				Usage: "Optional. treat as block device default false",
			},
		},
		Action: func(c *cli.Context) {
			if err := deleteLV(c); err != nil {
				klog.Fatalf("Error deleting lv: %v", err)
			}
		},
	}
}

func deleteLV(c *cli.Context) error {
	lvName := c.String(flagLVName)
	if lvName == "" {
		return fmt.Errorf("invalid empty flag %v", flagLVName)
	}
	vgName := c.String(flagVGName)
	if vgName == "" {
		return fmt.Errorf("invalid empty flag %v", flagVGName)
	}
	dirName := c.String(flagDirectory)
	if dirName == "" {
		return fmt.Errorf("invalid empty flag %v", flagDirectory)
	}
	blockMode := c.Bool(flagBlockMode)

	klog.Infof("delete lv %s vg:%s dir:%s block:%t", lvName, vgName, dirName, blockMode)

	if !blockMode {
		output, err := umountLV(lvName, vgName, dirName)
		if err != nil {
			return fmt.Errorf("unable to delete lv: %v output:%s", err, output)
		}
	}

	output, err := commands.RemoveLV(context.Background(), vgName, lvName)
	if err != nil {
		return fmt.Errorf("unable to delete lv: %v output:%s", err, output)
	}
	klog.Infof("lv %s vg:%s deleted", lvName, vgName)
	return nil
}

func umountLV(lvname, vgname, directory string) (string, error) {
	lvPath := fmt.Sprintf("/dev/%s/%s", vgname, lvname)
	mountPath := path.Join(directory, lvname)

	cmd := exec.Command("umount", "--lazy", "--force", lvPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("unable to umount %s from %s output:%s err:%v", lvPath, mountPath, string(out), err)
	}
	err = os.Remove(mountPath)
	if err != nil {
		klog.Errorf("unable to remove mount directory:%s err:%v", mountPath, err)
	}
	return "", nil
}
