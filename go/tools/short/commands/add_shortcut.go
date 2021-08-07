package commands

import (
	"fmt"
	"time"

	clilib "dfl/lib/cli"
	"dfl/tools/short/app"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var AddShortcut = &cli.Command{
	Name:      "add-shortcut",
	Usage:     "Add a shortcut",
	ArgsUsage: "[query] [shortcut]",
	Aliases:   []string{"asc"},

	Action: func(c *cli.Context) error {
		startTime := time.Now()

		app := c.Context.Value(clilib.AppKey).(*app.App)

		query, shortcut, err := handleShortcutInput(app, c.Args().Slice())
		if err != nil {
			return err
		}

		err = app.AddShortcut(c.Context, query, shortcut)
		if err != nil {
			return err
		}

		clilib.WriteClipboard(fmt.Sprintf("%s/:%s", app.RootURL, shortcut))
		clilib.Notify("Added shortcut", fmt.Sprintf("%s/:%s", app.RootURL, shortcut))

		log.Infof("Done in %s", time.Now().Sub(startTime))

		return nil
	},
}

func handleShortcutInput(app *app.App, args []string) (string, string, error) {
	if len(args) == 2 {
		return app.Trim(args[0]), args[1], nil
	}

	query, err := app.CleanInput([]string{})
	if err != nil {
		return "", "", err
	}

	shortcut, err := shortcutPrompt.Run()
	if err != nil {
		return "", "", err
	}

	return query, shortcut, nil
}
