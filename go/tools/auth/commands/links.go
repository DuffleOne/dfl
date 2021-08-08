package commands

import (
	"fmt"

	clilib "dfl/lib/cli"
	"dfl/tools/auth/app"

	"github.com/urfave/cli/v2"
)

var Manage = &cli.Command{
	Name:    "manage",
	Usage:   "Manage your U2F tokens",
	Aliases: []string{"m"},

	Action: func(c *cli.Context) error {
		app := c.Context.Value(clilib.AppKey).(*app.App)

		return openBrowser(fmt.Sprintf("%s/u2f_manage", app.RootURL))
	},
}

var Register = &cli.Command{
	Name:    "register",
	Usage:   "Register for an account",
	Aliases: []string{"r"},

	Action: func(c *cli.Context) error {
		app := c.Context.Value(clilib.AppKey).(*app.App)

		return openBrowser(fmt.Sprintf("%s/register", app.RootURL))
	},
}
