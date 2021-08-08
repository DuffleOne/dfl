package cmd

import (
	"dfl/svc/auth/server"

	"github.com/kelseyhightower/envconfig"
	"github.com/urfave/cli/v2"
)

// RootCmd is the default command for the upload proxy service executable.
var RootCmd = &cli.Command{
	Name:  "auth",
	Usage: "auth handles all auth related matters",

	Action: func(c *cli.Context) error {
		cfg := server.DefaultConfig()

		if err := envconfig.Process("auth", &cfg); err != nil {
			return err
		}

		return server.Run(cfg)
	},
}
