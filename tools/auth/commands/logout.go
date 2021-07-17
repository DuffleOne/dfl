package commands

import (
	clilib "dfl/lib/cli"
	"dfl/tools/auth/app"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/pterm/pterm"
	"github.com/urfave/cli/v2"
)

var Logout = &cli.Command{
	Name:  "logout",
	Usage: "Delete your local credentials",

	Action: func(c *cli.Context) error {
		app := c.Context.Value(clilib.AppKey).(*app.App)

		if err := app.Keychain.DeleteItem("Auth"); err != nil {
			if v, ok := err.(cher.E); ok && v.Code == cher.NotFound {
				return cher.New("not_logged_in", nil)
			}

			return err
		}

		pterm.Success.Println("Logged out!")

		return nil
	},
}
