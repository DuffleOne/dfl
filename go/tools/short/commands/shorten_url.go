package commands

import (
	"time"

	clilib "dfl/lib/cli"
	"dfl/tools/short/app"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var ShortenURL = &cli.Command{
	Name:      "shorten",
	ArgsUsage: "[url]",
	Aliases:   []string{"s"},
	Usage:     "Shorten a URL",

	Action: func(c *cli.Context) error {
		startTime := time.Now()

		app := c.Context.Value(clilib.AppKey).(*app.App)

		url, err := handleURLInput(c.Args().Slice())
		if err != nil {
			return err
		}

		body, err := app.ShortenURL(c.Context, url)
		if err != nil {
			return err
		}

		clilib.WriteClipboard(body.URL)
		clilib.Notify("URL Shortened", body.URL)

		log.Infof("Done in %s: %s", time.Now().Sub(startTime), body.URL)

		return nil
	},
}
