package commands

import (
	clilib "dfl/lib/cli"
	"dfl/tools/certgen/app"

	"github.com/urfave/cli/v2"
)

var GenerateKeyPairCmd = &cli.Command{
	Name:      "generate_key_pair",
	ArgsUsage: "[name]",
	Aliases:   []string{"gkp"},
	Usage:     "Generate a public and private key pair",

	Action: func(c *cli.Context) error {
		name := c.Args().First()

		app := c.Context.Value(clilib.AppKey).(*app.App)

		if err := app.GenerateKeyPair(name); err != nil {
			return err
		}

		return app.VerifyKeyPair(name)
	},
}
