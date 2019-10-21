package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path"

	"github.com/google/lvmd/commands"
	"github.com/urfave/cli"
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
		},
		Action: func(c *cli.Context) {
			if err := deleteLV(c); err != nil {
				log.Fatalf("Error deleting lv: %v", err)
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
	log.Printf("delete lv %s vg:%s dir:%s", lvName, vgName, dirName)

	output, err := umountLV(lvName, vgName, dirName)
	if err != nil {
		return fmt.Errorf("unable to delete lv: %v output:%s", err, output)
	}

	output, err = commands.RemoveLV(context.Background(), vgName, lvName)
	if err != nil {
		return fmt.Errorf("unable to delete lv: %v output:%s", err, output)
	}
	log.Printf("lv %s vg:%s deleted", lvName, vgName)
	return nil
}

func umountLV(lvname, vgname, directory string) (string, error) {
	lvPath := fmt.Sprintf("/dev/%s/%s", vgname, lvname)
	mountPath := path.Join(directory, lvname)

	cmd := exec.Command("umount", lvPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("unable to umount %s from %s err:%v", lvPath, mountPath, err)
	}
	return "", nil
}
