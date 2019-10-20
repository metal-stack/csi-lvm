package main

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli"
)

const (
	flagLVName = "lvname"
	flagLVSize = "lvsize"
	flagVGName = "vgname"
	flagPVS    = "pvs"
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

func cmdNotFound(c *cli.Context, command string) {
	panic(fmt.Errorf("Unrecognized command: %s", command))
}

func onUsageError(c *cli.Context, err error, isSubcommand bool) error {
	panic(fmt.Errorf("Usage error, please check your command"))
}

func main() {
	p := cli.NewApp()
	p.Usage = "LVM Provisioner Pod"
	p.Commands = []cli.Command{
		createLVCmd(),
	}
	p.CommandNotFound = cmdNotFound
	p.OnUsageError = onUsageError

	if err := p.Run(os.Args); err != nil {
		log.Fatalf("Critical error: %v", err)
	}
}
