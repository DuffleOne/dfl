package commands

import (
	clilib "dfl/lib/cli"
	"dfl/tools/certgen/app"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"
)

const (
	RootCA  = "Root CA"
	Server  = "Server"
	Client  = "Client"
	KeyPair = "Key pair"
	CRL     = "Revocation list"
)

var InteractiveCmd = &cli.Command{
	Name:    "interactive",
	Aliases: []string{"i"},
	Usage:   "Start an interactive console",

	Action: func(c *cli.Context) error {
		app := c.Context.Value(clilib.AppKey).(*app.App)

		_, certType, err := certificateTypesPrompt.Run()
		if err != nil {
			return err
		}

		switch certType {
		case RootCA:
			_, err := confirmPrompt.Run()
			if err != nil {
				if err.Error() == "" {
					return nil
				}

				return err
			}

			return app.GenerateRootCA()
		case CRL:
			_, err := confirmPrompt.Run()
			if err != nil {
				if err.Error() == "" {
					return nil
				}

				return err
			}

			return app.GenerateCRL()
		case Server:
			certName, err := namePrompt.Run()
			if err != nil {
				return err
			}

			_, err = confirmPrompt.Run()
			if err != nil {
				if err.Error() == "" {
					return nil
				}

				return err
			}

			return app.GenerateServerCertificate(certName)
		case Client:
			certName, err := namePrompt.Run()
			if err != nil {
				return err
			}

			password, err := passwordPrompt.Run()
			if err != nil {
				return err
			}

			_, err = confirmPrompt.Run()
			if err != nil {
				if err.Error() == "" {
					return nil
				}

				return err
			}

			return app.GenerateClientCertificate(certName, password)
		case KeyPair:
			certName, err := namePrompt.Run()
			if err != nil {
				return err
			}

			_, err = confirmPrompt.Run()
			if err != nil {
				if err.Error() == "" {
					return nil
				}

				return err
			}

			return app.GenerateKeyPair(certName)
		default:
			return cher.New("invalid_certificate_type", cher.M{"type": certType})
		}
	},
}

var certificateTypesPrompt = promptui.Select{
	Label: "Certificate to generate",
	Items: []string{RootCA, Server, Client, KeyPair, CRL},
}

var namePrompt = promptui.Prompt{
	Label: "Name",
	Validate: func(input string) error {
		if len(input) < 3 {
			return cher.New("name_too_short", nil)
		}

		return nil
	},
}

var passwordPrompt = promptui.Prompt{
	Label:       "Password",
	Mask:        '*',
	HideEntered: true,
	Validate: func(input string) error {
		if len(input) == 0 {
			return cher.New("password_too_short", nil)
		}

		return nil
	},
}

var confirmPrompt = promptui.Prompt{
	Label:     "Create certificate",
	IsConfirm: true,
}
