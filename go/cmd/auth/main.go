package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	clilib "dfl/lib/cli"
	"dfl/lib/keychain"
	"dfl/tools/auth/app"
	"dfl/tools/auth/commands"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const clientID = "client_000000C3N8sN2HPqTVeqfOTsnjBJI"

func main() {
	if err := rootCmd.Run(os.Args); err != nil {
		if v, ok := err.(cher.E); ok {
			bytes, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(string(bytes))
		} else {
			log.Fatal(err)
		}
	}
}

var rootCmd *cli.App

func makeRoot(kc keychain.Keychain) {
	rootCmd = &cli.App{
		Name:  "auth",
		Usage: "Manage your authentication to DFL services",

		Commands: []*cli.Command{
			commands.CreateInviteCode,
			commands.Login(clientID, "auth:login"),
			commands.Logout,
			commands.Manage,
			commands.Register,
			commands.SetToken,
			commands.ShowAccessToken,
			commands.WhoAmI,
		},

		Before: func(c *cli.Context) error {
			if len(c.Args().Slice()) == 0 {
				return nil
			}

			var config clilib.Config

			if err := envconfig.Process("DFL", &config); err != nil {
				return err
			}

			app, err := app.New(config.AuthURL, config.AuthUI, kc)
			if err != nil {
				return err
			}

			c.Context = context.WithValue(c.Context, clilib.AppKey, app)
			c.Context = context.WithValue(c.Context, clilib.ConfigKey, config)

			return nil
		},
	}
}
