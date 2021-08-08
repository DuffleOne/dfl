package commands

import (
	"crypto/x509"

	clilib "dfl/lib/cli"
	"dfl/tools/certgen/app"

	"github.com/urfave/cli/v2"
)

var filename = "crl.txt"

var crlTemplate = &x509.RevocationList{
	SignatureAlgorithm: x509.ECDSAWithSHA384,
}

var CreateCRLFileCmd = &cli.Command{
	Name:    "create_crl_file",
	Aliases: []string{"crl"},
	Usage:   "Create a CRL file",

	Action: func(c *cli.Context) error {
		app := c.Context.Value(clilib.AppKey).(*app.App)

		return app.GenerateCRL()
	},
}
