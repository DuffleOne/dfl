package commands

import (
	"time"

	clilib "dfl/lib/cli"
	"dfl/tools/short/app"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var DeleteResource = &cli.Command{
	Name:      "delete",
	ArgsUsage: "[query]",
	Aliases:   []string{"d"},
	Usage:     "Delete a resource",

	Action: func(c *cli.Context) error {
		startTime := time.Now()

		app := c.Context.Value(clilib.AppKey).(*app.App)

		query, err := app.CleanInput(c.Args().Slice())
		if err != nil {
			return err
		}

		err = app.DeleteResource(c.Context, query)
		if err != nil {
			return err
		}

		clilib.Notify("Resource deleted", query)

		log.Infof("Done in %s", time.Now().Sub(startTime))

		return nil
	},
}
