package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	clilib "dfl/lib/cli"
	"dfl/tools/certgen/app"
	"dfl/tools/certgen/commands"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	if err := rootCmd.Run(os.Args); err != nil {
		if c, ok := err.(cher.E); ok {
			bytes, err := json.MarshalIndent(c, "", "  ")
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(string(bytes))
			os.Exit(1)
		}

		log.Fatal(err)
	}
}

var rootCmd = &cli.App{
	Name:  "certgen",
	Usage: "certgen manages and generates certificates for you.",

	Commands: []*cli.Command{
		commands.CreateCRLFileCmd,
		commands.GenerateClientCertificateCmd,
		commands.GenerateKeyPairCmd,
		commands.GenerateRootCACmd,
		commands.GenerateServerCertificateCmd,
		commands.InteractiveCmd,
		commands.VersionCmd,
	},

	Before: func(c *cli.Context) error {
		var config app.Config

		if err := envconfig.Process("certgen", &config); err != nil {
			log.Fatal(err)
		}

		app := &app.App{
			RootDirectory: config.RootDir,
		}

		c.Context = context.WithValue(c.Context, clilib.AppKey, app)
		c.Context = context.WithValue(c.Context, clilib.ConfigKey, config)

		return nil
	},
}
