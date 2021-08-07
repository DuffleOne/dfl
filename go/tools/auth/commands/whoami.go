package commands

import (
	"fmt"

	clilib "dfl/lib/cli"
	"dfl/tools/auth/app"

	"github.com/urfave/cli/v2"
)

var WhoAmI = &cli.Command{
	Name:  "whoami",
	Usage: "Read the access token and find out what your ID and username is",

	Action: func(c *cli.Context) error {
		app := c.Context.Value(clilib.AppKey).(*app.App)

		whoami, err := app.Client.WhoAmI(c.Context)
		if err != nil {
			return err
		}

		fmt.Println("User ID:  ", whoami.UserID)
		fmt.Println("Username: ", whoami.Username)

		return nil
	},
}
