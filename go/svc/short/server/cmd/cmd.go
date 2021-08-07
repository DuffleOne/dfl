package cmd

import (
	"dfl/svc/short/server"

	"github.com/kelseyhightower/envconfig"
	"github.com/urfave/cli/v2"
)

// RootCmd is the default command for the upload proxy service executable.
var RootCmd = &cli.Command{
	Name:  "short",
	Usage: "short handles all short URLs for files and URLs",

	Action: func(c *cli.Context) error {
		cfg := server.DefaultConfig()

		if err := envconfig.Process("short", &cfg); err != nil {
			return err
		}

		return server.Run(cfg)
	},
}
