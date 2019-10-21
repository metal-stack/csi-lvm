package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/google/lvmd/commands"
	"github.com/urfave/cli"
)

func createLVCmd() cli.Command {
	return cli.Command{
		Name: "createlv",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  flagLVName,
				Usage: "Required. Specify lv name.",
			},
			cli.Uint64Flag{
				Name:  flagLVSize,
				Usage: "Required. The size of the lv in MiB",
			},
			cli.StringFlag{
				Name:  flagVGName,
				Usage: "Required. the name of the volumegroup",
			},
			cli.StringFlag{
				Name:  flagDirectory,
				Usage: "Required. the name of the directory to mount the lv",
			},
			cli.StringSliceFlag{
				Name:  flagDevicesPattern,
				Usage: "Required. the patterns of the physical volumes to use.",
			},
		},
		Action: func(c *cli.Context) {
			if err := createLV(c); err != nil {
				log.Fatalf("Error creating lv: %v", err)
			}
		},
	}
}

func createLV(c *cli.Context) error {
	lvName := c.String(flagLVName)
	if lvName == "" {
		return fmt.Errorf("invalid empty flag %v", flagLVName)
	}
	lvSize := c.Uint64(flagLVSize)
	if lvSize == 0 {
		return fmt.Errorf("invalid empty flag %v", flagLVSize)
	}
	vgName := c.String(flagVGName)
	if vgName == "" {
		return fmt.Errorf("invalid empty flag %v", flagVGName)
	}
	dirName := c.String(flagDirectory)
	if dirName == "" {
		return fmt.Errorf("invalid empty flag %v", flagDirectory)
	}
	devicesPattern := c.StringSlice(flagDevicesPattern)
	if len(devicesPattern) == 0 {
		return fmt.Errorf("invalid empty flag %v", flagDevicesPattern)
	}

	log.Printf("create lv %s size:%d vg:%s devices:%s dir:%s", lvName, lvSize, vgName, devicesPattern, dirName)

	vgs, err := commands.ListVG(context.Background())
	if err != nil {
		log.Printf("unable to list existing volumegroups:%v", err)
	}
	vgexists := false
	for _, vg := range vgs {
		if vg.Name == vgName {
			vgexists = true
			break
		}
	}
	if !vgexists {
		devs, err := devices(devicesPattern)
		if err != nil {
			return fmt.Errorf("unable to lookup devices from devicesPattern %s, err:%v", devicesPattern, err)
		}
		tags := []string{"vg.metal-pod.io/csi-lvm"}
		output, err := createVG(vgName, devs, tags)
		if err != nil {
			return fmt.Errorf("unable to create vg: %v output:%s", err, output)
		}
	}
	tags := []string{"lv.metal-pod.io/csi-lvm"}
	output, err := commands.CreateLV(context.Background(), vgName, lvName, lvSize, 0, tags)
	if err != nil {
		return fmt.Errorf("unable to create lv: %v output:%s", err, output)
	}

	// output, err = mountLV(lvName, vgName, dirName)
	output, err = mountLV(lvName, vgName, "/data")
	if err != nil {
		return fmt.Errorf("unable to mount lv: %v output:%s", err, output)
	}

	log.Printf("lv %s size:%d vg:%s devices:%s created", lvName, lvSize, vgName, devicesPattern)
	return nil
}

func devices(devicesPattern []string) (devices []string, err error) {
	for _, devicePattern := range devicesPattern {
		log.Printf("search devices :%s ", devicePattern)
		matches, err := filepath.Glob(devicePattern)
		if err != nil {
			return nil, err
		}
		log.Printf("found: %s", matches)
		devices = append(devices, matches...)
	}
	return devices, nil
}

func mountLV(lvname, vgname, directory string) (string, error) {
	lvPath := fmt.Sprintf("/dev/%s/%s", vgname, lvname)
	mountPath := path.Join(directory, lvname)
	cmd := exec.Command("mkfs.ext4", lvPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("unable to format lv:%s err:%v", lvname, err)
	}

	err = os.MkdirAll(mountPath, 0777)
	if err != nil {
		return string(out), fmt.Errorf("unable to create mount directory for lv:%s err:%v", lvname, err)
	}

	cmd = exec.Command("mount", "-t", "ext4", lvPath, mountPath)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("unable to mount %s to %s err:%v", lvPath, mountPath, err)
	}
	return "", nil
}

func createVG(name string, physicalVolumes []string, tags []string) (string, error) {
	args := []string{"-v", name}
	args = append(args, physicalVolumes...)
	for _, tag := range tags {
		args = append(args, "--add-tag", tag)
	}
	log.Printf("create vg with command: vgcreate %v", args)
	cmd := exec.Command("vgcreate", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
