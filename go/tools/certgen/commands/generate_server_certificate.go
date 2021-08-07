package commands

import (
	clilib "dfl/lib/cli"
	"dfl/tools/certgen/app"

	"github.com/urfave/cli/v2"
)

var GenerateServerCertificateCmd = &cli.Command{
	Name:      "generate_server_ceritificate",
	ArgsUsage: "[domain]",
	Aliases:   []string{"gsc"},
	Usage:     "Generate a server certificate",

	Action: func(c *cli.Context) error {
		name := c.Args().First()

		app := c.Context.Value(clilib.AppKey).(*app.App)

		return app.GenerateServerCertificate(name)
	},
}
