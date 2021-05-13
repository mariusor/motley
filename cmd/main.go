package main

import (
	"fmt"
	"os"

	"git.sr.ht/~marius/motley/internal/cmd"
	"git.sr.ht/~marius/motley/internal/config"
	"git.sr.ht/~marius/motley/internal/env"
	"gopkg.in/urfave/cli.v2"
)

var version = "HEAD"

func main() {
	app := cli.App{}
	app.Name = "motley"
	app.Usage = "helper utility to manage a FedBOX instance"
	app.Version = version
	app.Before = cmd.Before
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "url",
			Usage: "The url used by the application (REQUIRED)",
		},
		&cli.StringFlag{
			Name:  "env",
			Usage: fmt.Sprintf("The environment to use. Possible values: %q", []env.Type{env.DEV, env.QA, env.PROD}),
			Value: string(env.DEV),
		},
		&cli.StringFlag{
			Name:  "type",
			Usage: fmt.Sprintf("Type of the backend to use. Possible values: %q", []config.StorageType{config.StorageBoltDB, config.StorageBadger, config.StorageFS}),
		},
		&cli.StringFlag{
			Name:  "path",
			Value: ".",
			Usage: fmt.Sprintf("The path for the storage folder orsocket"),
		},
		&cli.StringFlag{
			Name:  "user",
			Value: "fedbox",
			Usage: "The postgres database user",
		},
	}
	app.Action = cmd.TuiAction

	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
