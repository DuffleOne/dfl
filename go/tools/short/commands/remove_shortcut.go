package commands

import (
	"time"

	clilib "dfl/lib/cli"
	"dfl/tools/short/app"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var RemoveShortcut = &cli.Command{
	Name:      "remove-shortcut",
	ArgsUsage: "[query] [shortcut]",
	Aliases:   []string{"rsc"},
	Usage:     "Remove a shortcut",

	Action: func(c *cli.Context) error {
		startTime := time.Now()

		app := c.Context.Value(clilib.AppKey).(*app.App)

		query, shortcut, err := handleShortcutInput(app, c.Args().Slice())
		if err != nil {
			return err
		}

		err = app.RemoveShortcut(c.Context, query, shortcut)
		if err != nil {
			return err
		}

		clilib.Notify("Removed shortcut", shortcut)

		log.Infof("Done in %s", time.Now().Sub(startTime))

		return nil
	},
}
