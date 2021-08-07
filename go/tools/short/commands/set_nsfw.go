package commands

import (
	"time"

	clilib "dfl/lib/cli"
	"dfl/tools/short/app"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var SetNSFW = &cli.Command{
	Name:      "nsfw",
	ArgsUsage: "[query]",
	Aliases:   []string{"n"},
	Usage:     "Toggle the NSFW flag",

	Action: func(c *cli.Context) error {
		startTime := time.Now()

		app := c.Context.Value(clilib.AppKey).(*app.App)

		query, err := app.CleanInput(c.Args().Slice())
		if err != nil {
			return err
		}

		newState := "ON"

		newValue, err := app.ToggleNSFW(c.Context, query)
		if err != nil {
			return err
		}

		if !newValue {
			newState = "OFF"
		}

		log.Infof("NSFW flag is now %s", newState)

		log.Infof("Done in %s", time.Now().Sub(startTime))

		return nil
	},
}
