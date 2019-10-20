package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"

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
			// FIXME lvmd does not allow to create vgs with multiple pvs
			cli.StringSliceFlag{
				Name:  flagPVS,
				Usage: "Required. the list of the physical volumes to use",
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
	lvSize := c.Uint64(flagLVSize) * 1024 * 1024
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
	pvs := c.StringSlice(flagPVS)
	if len(pvs) == 0 {
		return fmt.Errorf("invalid empty flag %v", flagPVS)
	}

	log.Printf("create lv %s size:%d vg:%s pvs:%s dir:%s", lvName, lvSize, vgName, pvs, dirName)

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
		tags := []string{"vg.metal-pod.io/csi-lvm"}
		output, err := commands.CreateVG(context.Background(), vgName, pvs[0], tags)
		if err != nil {
			return fmt.Errorf("unable to create vg: %v output:%s", err, output)
		}
	}
	tags := []string{"lv.metal-pod.io/csi-lvm"}
	output, err := commands.CreateLV(context.Background(), vgName, lvName, lvSize, 0, tags)
	if err != nil {
		return fmt.Errorf("unable to create lv: %v output:%s", err, output)
	}

	output, err = mountLV(lvName, vgName, dirName)
	if err != nil {
		return err
	}

	log.Printf("lv %s size:%d vg:%s pvs:%s created", lvName, lvSize, vgName, pvs)
	return nil
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
