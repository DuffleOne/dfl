package main

import (
	"os"

	"dfl/lib/config"

	auth "dfl/svc/auth/server/cmd"
	monitor "dfl/svc/monitor/server/cmd"
	short "dfl/svc/short/server/cmd"

	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	"github.com/cuvva/cuvva-public-go/lib/servicecontext"
	"github.com/urfave/cli/v2"
)

// RootCmd is the initial entrypoint where all services are mounted.
var RootCmd = &cli.App{
	Name:  "dfl",
	Usage: "dfl monobinary for dfl monorepo",

	Commands: []*cli.Command{
		auth.RootCmd,
		monitor.RootCmd,
		short.RootCmd,
	},

	Before: func(c *cli.Context) error {
		env := config.EnvironmentName(os.Getenv)

		ksuid.SetEnvironment(env)
		servicecontext.Set(c.Command.Name, env)

		return nil
	},
}

func main() {
	if err := RootCmd.Run(os.Args); err != nil {
		panic(err)
	}
}
