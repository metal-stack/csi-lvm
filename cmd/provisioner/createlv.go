package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/lvmd/commands"
	"github.com/urfave/cli"
)

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
	pvs := c.StringSlice(flagPVS)
	if len(pvs) == 0 {
		return fmt.Errorf("invalid empty flag %v", flagPVS)
	}

	log.Printf("create lv %s size:%d vg:%s pvs:%s", lvName, lvSize, vgName, pvs)

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
	tags := []string{"vg.metal-pod.io/csi-lvm"}
	output, err := commands.CreateLV(context.Background(), vgName, lvName, lvSize, 0, tags)
	if err != nil {
		return fmt.Errorf("unable to create lv: %v output:%s", err, output)
	}
	log.Printf("lv %s size:%d vg:%s pvs:%s created", lvName, lvSize, vgName, pvs)
	return nil
}
