package commands

import (
	clilib "dfl/lib/cli"
	"dfl/tools/certgen/app"

	"github.com/urfave/cli/v2"
)

var GenerateRootCACmd = &cli.Command{
	Name:    "generate_root_ca",
	Aliases: []string{"gca"},
	Usage:   "Generate a new root CA",

	Action: func(c *cli.Context) error {
		app := c.Context.Value(clilib.AppKey).(*app.App)

		return app.GenerateRootCA()
	},
}
