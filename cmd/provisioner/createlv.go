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
				Usage: "Required. type of lvs, can be either striped or mirrored",
			},
			cli.StringSliceFlag{
				Name:  flagDevicesPattern,
				Usage: "Required. the patterns of the physical volumes to use.",
			},
			cli.BoolFlag{
				Name:  flagBlockMode,
				Usage: "Optional. create a block device only, default false",
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
	if lvmType == "" {
		return fmt.Errorf("invalid empty flag %v", flagLVMType)
	}
	blockMode := c.Bool(flagBlockMode)

	log.Printf("create lv %s size:%d vg:%s devicespattern:%s dir:%s type:%s block:%t", lvName, lvSize, vgName, devicesPattern, dirName, lvmType, blockMode)

	output, err := createVG(vgName, devicesPattern)
	if err != nil {
		return fmt.Errorf("unable to create vg: %v output:%s", err, output)
	}

	output, err = createLVS(context.Background(), vgName, lvName, lvSize, lvmType)
	if err != nil {
		return fmt.Errorf("unable to create lv: %v output:%s", err, output)
	}

	if !blockMode {
		output, err = mountLV(lvName, vgName, dirName)
		if err != nil {
			return fmt.Errorf("unable to mount lv: %v output:%s", err, output)
		}
		log.Printf("mounted lv %s size:%d vg:%s devices:%s created", lvName, lvSize, vgName, devicesPattern)
	} else {
		log.Printf("block lv %s size:%d vg:%s devices:%s created", lvName, lvSize, vgName, devicesPattern)
	}

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
	// check for format with blkid /dev/csi-lvm/pvc-xxxxx
	// /dev/dm-3: UUID="d1910e3a-32a9-48d2-aa2e-e5ad018237c9" TYPE="ext4"
	lvPath := fmt.Sprintf("/dev/%s/%s", vgname, lvname)

	formatted := false
	// check for already formatted
	cmd := exec.Command("blkid", lvPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("unable to check if %s is already formatted:%v", lvPath, err)
	}
	if strings.Contains(string(out), "ext4") {
		formatted = true
	}

	mountPath := path.Join(directory, lvname)
	if !formatted {
		log.Printf("formatting with mkfs.ext4 %s", lvPath)
		cmd = exec.Command("mkfs.ext4", lvPath)
		out, err = cmd.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("unable to format lv:%s err:%v", lvname, err)
		}
	}

	err = os.MkdirAll(mountPath, 0777)
	if err != nil {
		return string(out), fmt.Errorf("unable to create mount directory for lv:%s err:%v", lvname, err)
	}

	// --make-shared is required that this mount is visible outside this container.
	mountArgs := []string{"--make-shared", "-t", "ext4", lvPath, mountPath}
	log.Printf("mountlv command: mount %s", mountArgs)
	cmd = exec.Command("mount", mountArgs...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		mountOutput := string(out)
		if !strings.Contains(mountOutput, "already mounted") {
			return string(out), fmt.Errorf("unable to mount %s to %s err:%v output:%s", lvPath, mountPath, err, out)
		}
	}
	log.Printf("mountlv output:%s", out)
	return "", nil
}

func createVG(name string, devicesPattern []string) (string, error) {
	vgs, err := commands.ListVG(context.Background())
	if err != nil {
		log.Printf("unable to list existing volumegroups:%v", err)
	}
	vgexists := false
	for _, vg := range vgs {
		log.Printf("compare vg:%s with:%s\n", vg.Name, name)
		if vg.Name == name {
			vgexists = true
			break
		}
	}
	if vgexists {
		log.Printf("volumegroup: %s already exists\n", name)
		return name, nil
	}
	physicalVolumes, err := devices(devicesPattern)
	if err != nil {
		return "", fmt.Errorf("unable to lookup devices from devicesPattern %s, err:%v", devicesPattern, err)
	}
	tags := []string{"vg.metal-pod.io/csi-lvm"}

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
func createLVS(ctx context.Context, vg string, name string, size uint64, lvmType string) (string, error) {
	lvs, err := commands.ListLV(context.Background(), vg+"/"+name)
	if err != nil {
		log.Printf("unable to list existing logicalvolumes:%v", err)
	}
	lvExists := false
	for _, lv := range lvs {
		log.Printf("compare lv:%s with:%s\n", lv.Name, name)
		if strings.Contains(lv.Name, name) {
			lvExists = true
			break
		}
	}

	if lvExists {
		log.Printf("logicalvolume: %s already exists\n", name)
		return name, nil
	}

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
		return "", fmt.Errorf("unsupported lvmtype: %s", lvmType)
	}

	tags := []string{"lv.metal-pod.io/csi-lvm"}
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
