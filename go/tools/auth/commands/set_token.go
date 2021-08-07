package commands

import (
	authlib "dfl/lib/auth"
	clilib "dfl/lib/cli"
	"dfl/svc/auth"
	"dfl/tools/auth/app"
	"encoding/json"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/golang-jwt/jwt"
	"github.com/urfave/cli/v2"
)

var SetToken = &cli.Command{
	Name:      "set-access-token",
	Usage:     "Manually set the access token",
	ArgsUsage: "[token]",

	Action: func(c *cli.Context) error {
		var dflclaims authlib.DFLClaims

		if token, _ := jwt.ParseWithClaims(c.Args().First(), &dflclaims, nil); token == nil {
			return cher.New("cannot_parse_token", nil)
		}

		res := auth.TokenResponse{
			UserID:      dflclaims.Subject,
			AccessToken: c.Args().First(),
			TokenType:   "Bearer",
			Expires:     int(dflclaims.ExpiresAt),
		}

		authBytes, err := json.Marshal(res)
		if err != nil {
			return err
		}

		app := c.Context.Value(clilib.AppKey).(*app.App)

		return app.Keychain.UpsertItem("Auth", authBytes)
	},
}
