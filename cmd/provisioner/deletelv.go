package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/google/lvmd/commands"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func deleteLVCmd() *cli.Command {
	return &cli.Command{
		Name: "deletelv",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  flagLVName,
				Usage: "Required. Specify lv name.",
			},
			&cli.StringFlag{
				Name:  flagVGName,
				Usage: "Required. the name of the volumegroup",
			},
			&cli.StringFlag{
				Name:  flagDirectory,
				Usage: "Required. the name of the directory to mount the lv",
			},
			&cli.BoolFlag{
				Name:  flagBlockMode,
				Usage: "Optional. treat as block device default false",
			},
		},
		Action: func(c *cli.Context) error {
			if err := deleteLV(c); err != nil {
				klog.Fatalf("Error deleting lv: %v", err)
				return err
			}
			return nil
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

	found := false
	lvs, err := commands.ListLV(context.Background(), vgName)
	if err != nil {
		return fmt.Errorf("unable to list existing logicalvolumes:%v", err)
	}
	for _, lv := range lvs {
		if strings.Contains(lv.Name, lvName) {
			found = true
			break
		}
	}
	if !found {
		klog.Infof("lv %s not found anymore", lvName)
		return nil
	}

	umountLV(lvName, vgName, dirName)

	output, err := commands.RemoveLV(context.Background(), vgName, lvName)
	if err != nil {
		return fmt.Errorf("unable to delete lv: %w output:%s", err, output)
	}
	klog.Infof("lv %s vg:%s deleted", lvName, vgName)
	return nil
}

func umountLV(lvname, vgname, directory string) {
	lvPath := fmt.Sprintf("/dev/%s/%s", vgname, lvname)
	mountPath := path.Join(directory, lvname)

	if _, err := os.Stat(mountPath); errors.Is(err, os.ErrNotExist) {
		klog.Infof("mount point %s not found anymore", mountPath)
		return
	}
	cmd := exec.Command("umount", "--lazy", "--force", mountPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("unable to umount %s from %s output:%s err:%w", mountPath, lvPath, string(out), err)
	}
	err = os.Remove(mountPath)
	if err != nil {
		klog.Errorf("unable to remove mount directory:%s err:%w", mountPath, err)
	}
}
