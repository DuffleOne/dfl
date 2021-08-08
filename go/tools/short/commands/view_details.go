package commands

import (
	"encoding/json"
	"fmt"

	clilib "dfl/lib/cli"
	"dfl/svc/short"
	"dfl/tools/short/app"

	"github.com/urfave/cli/v2"
)

var ViewDetails = &cli.Command{
	Name:      "view",
	ArgsUsage: "[query]",
	Aliases:   []string{"v"},
	Usage:     "View details of a resource",

	Action: func(c *cli.Context) error {
		app := c.Context.Value(clilib.AppKey).(*app.App)

		query, err := app.CleanInput(c.Args().Slice())
		if err != nil {
			return err
		}

		res, err := app.Client.ViewDetails(c.Context, &short.IdentifyResource{
			Query: query,
		})
		if err != nil {
			return err
		}

		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(b))

		return nil
	},
}
