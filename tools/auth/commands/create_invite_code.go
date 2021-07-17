package commands

import (
	"fmt"
	"regexp"
	"time"

	clilib "dfl/lib/cli"
	"dfl/svc/auth"
	"dfl/tools/auth/app"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/manifoldco/promptui"
	"github.com/pterm/pterm"
	"github.com/urfave/cli/v2"
)

var scopesRegex = regexp.MustCompile(`^(?:[a-z*]+:[a-z*]+\s?)+$`)

var CreateInviteCode = &cli.Command{
	Name:    "create-invite-code",
	Usage:   "Create an invite code for someone else",
	Aliases: []string{"cic"},

	Action: func(c *cli.Context) (err error) {
		scopes, code, expiresAt, err := handleInputs()
		if err != nil {
			return err
		}

		spinner, err := pterm.DefaultSpinner.Start("Creating an invite code")
		if err != nil {
			return err
		}

		app := c.Context.Value(clilib.AppKey).(*app.App)

		res, err := app.Client.CreateInviteCode(c.Context, &auth.CreateInviteCodeRequest{
			Code:      code,
			ExpiresAt: expiresAt,
			Scopes:    scopes,
		})
		if err != nil {
			spinner.Fail()
			return err
		}

		spinner.UpdateText(fmt.Sprintf("Code: %s", res.Code))
		spinner.Success()

		if res.ExpiresAt != nil {
			fmt.Printf("Expires at: %s\n", res.ExpiresAt.Format(time.RFC3339))
		}

		return nil
	},
}

func handleInputs() (scopes string, code *string, expiresAt *time.Time, err error) {
	scopes, err = scopesPrompt.Run()
	if err != nil {
		return "", nil, nil, err
	}

	codeStr, err := codePrompt.Run()
	if err != nil {
		return "", nil, nil, err
	}

	expiresAtStr, err := expiresAtPrompt.Run()
	if err != nil {
		return "", nil, nil, err
	}

	if codeStr != "" {
		code = &codeStr
	}

	if expiresAtStr != "" {
		expiresAtTS, _ := time.Parse(time.RFC3339, expiresAtStr)
		expiresAt = &expiresAtTS
	}

	return
}

var scopesPrompt = promptui.Prompt{
	Label: "Scopes",
	Validate: func(in string) error {
		if len(in) == 0 {
			return cher.New("too_short", nil)
		}

		if !scopesRegex.MatchString(in) {
			return cher.New("invalid_scope_format", nil)
		}

		return nil
	},
}

var codePrompt = promptui.Prompt{
	Label: "Code",
	Validate: func(in string) error {
		return nil
	},
}

var expiresAtPrompt = promptui.Prompt{
	Label: "Expires at",
	Validate: func(in string) error {
		if len(in) == 0 {
			return nil
		}

		_, err := time.Parse(time.RFC3339, in)
		return err
	},
}
