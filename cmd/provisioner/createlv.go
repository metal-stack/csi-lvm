package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/lvmd/commands"
	"github.com/urfave/cli"
)

const (
	linearType  = "linear"
	stripedType = "striped"
	mirrorType  = "mirror"
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
			cli.StringFlag{
				Name:  flagLVMType,
				Usage: "Optional. type of lvs, can be either striped or mirrored",
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
	lvmType := c.String(flagLVMType)

	log.Printf("create lv %s size:%d vg:%s devices:%s dir:%s type:%s", lvName, lvSize, vgName, devicesPattern, dirName, lvmType)

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
	output, err := createLVS(context.Background(), vgName, lvName, lvSize, lvmType, tags)
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

	mountArgs := []string{"--make-shared", "-t", "ext4", lvPath, mountPath}
	log.Printf("mountlv command: mount %s", mountArgs)
	cmd = exec.Command("mount", mountArgs...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("unable to mount %s to %s err:%v output:%s", lvPath, mountPath, err, out)
	}
	log.Printf("mountlv output:%s", out)
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

// createLV creates a new volume
func createLVS(ctx context.Context, vg string, name string, size uint64, lvmType string, tags []string) (string, error) {
	if size == 0 {
		return "", fmt.Errorf("size must be greater than 0")
	}

	args := []string{"-v", "-n", name, "-W", "y", "-L", fmt.Sprintf("%db", size)}

	pvs, err := pvCount(vg)
	if err != nil {
		return "", fmt.Errorf("unable to determine pv count of vg: %v", err)
	}
	switch lvmType {
	case stripedType:
		if pvs < 2 {
			return "", fmt.Errorf("cannot use type %s when pv count is smaller than 2", lvmType)
		}
		args = append(args, "--type", "striped", "--stripes", fmt.Sprintf("%d", pvs))
	case mirrorType:
		if pvs < 2 {
			return "", fmt.Errorf("cannot use type %s when pv count is smaller than 2", lvmType)
		}
		args = append(args, "--type", "raid1", "--mirrors", "1", "--nosync")
	case linearType:
	default:
		return "", fmt.Errorf("unsupport lvmtype: %s", lvmType)
	}

	for _, tag := range tags {
		args = append(args, "--add-tag", tag)
	}
	args = append(args, vg)
	log.Printf("lvreate %s", args)
	cmd := exec.Command("lvcreate", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func pvCount(vgname string) (int, error) {
	cmd := exec.Command("vgs", vgname, "--noheadings", "-o", "pv_count")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}
	outStr := strings.TrimSpace(string(out))
	count, err := strconv.Atoi(outStr)
	if err != nil {
		return 0, err
	}
	return count, nil
}
