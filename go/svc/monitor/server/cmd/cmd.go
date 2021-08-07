package cmd

import (
	"dfl/svc/monitor/server"

	"github.com/kelseyhightower/envconfig"
	"github.com/urfave/cli/v2"
)

var RootCmd = &cli.Command{
	Name:  "monitor",
	Usage: "monitor handles all the service monitoring",

	Action: func(c *cli.Context) error {
		cfg := server.DefaultConfig()

		if err := envconfig.Process("monitor", &cfg); err != nil {
			return err
		}

		return server.Run(cfg)
	},
}
