package commands

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"

	clilib "dfl/lib/cli"
	"dfl/lib/keychain"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/dvsekhvalnov/jose2go/base64url"
	"github.com/manifoldco/promptui"
	"github.com/pterm/pterm"
	"github.com/tjarratt/babble"
	"github.com/urfave/cli/v2"
)

func Login(clientID, scopes string) *cli.Command {
	cmd := &cli.Command{
		Name:    "login",
		Usage:   "Login to the DFL auth server",
		Aliases: []string{"l"},

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "scopes",
				Usage:       "Which scopes should we request?",
				Value:       scopes,
				Destination: &scopes,
			},
		},

		Action: func(c *cli.Context) error {
			app := c.Context.Value(clilib.AppKey).(App)

			original, hashed := makeCodeChallenge()
			state := makeState()

			params := url.Values{
				"client_id":             []string{clientID},
				"scope":                 []string{scopes},
				"response_type":         []string{"code"},
				"state":                 []string{state},
				"nonce":                 []string{makeNonce()},
				"code_challenge_method": []string{"S256"},
				"code_challenge":        []string{hashed},
			}

			url := fmt.Sprintf("%s/authorize?%s", app.GetAuthURL(), params.Encode())

			pterm.Warning.Println("Ensure the state matches: ")
			pterm.Info.Println(state)

			err := openBrowser(url)
			if err != nil {
				pterm.Warning.Println("Cannot open your browser for you, type in the URL yourself.")
			}

			authToken, err := authTokenPrompt.Run()
			if err != nil {
				return err
			}

			spinner, err := pterm.DefaultSpinner.Start("Logging in")
			if err != nil {
				return err
			}

			res, err := app.GetAuthClient().Token(c.Context, &auth.TokenRequest{
				ClientID:     clientID,
				GrantType:    "authorization_code",
				Code:         authToken,
				CodeVerifier: original,
			})
			if err != nil {
				return err
			}

			authBytes, err := json.Marshal(res)
			if err != nil {
				return err
			}

			if err := app.GetKeychain().UpsertItem("Auth", authBytes); err != nil {
				return err
			}

			spinner.Success("Logged in!")

			return nil
		},
	}

	return cmd
}

var authTokenPrompt = promptui.Prompt{
	Label: "Auth token",
	Validate: func(in string) error {
		if len(in) == 0 {
			return cher.New("too_short", nil)
		}

		return nil
	},
}

func makeState() string {
	babbler := babble.NewBabbler()
	babbler.Count = 4

	return babbler.Babble()
}

func makeCodeChallenge() (original, hashed string) {
	randomBytes, err := generateRandomBytes(32)
	if err != nil {
		panic(err)
	}

	original = base64url.Encode(randomBytes)

	h := sha256.New()
	h.Write([]byte(original))
	hashed = base64url.Encode(h.Sum(nil))

	return
}

func makeNonce() string {
	bytes, err := generateRandomBytes(32)
	if err != nil {
		panic(err)
	}

	return base64url.Encode(bytes)
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func openBrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
}

// JUST for this command
// Because this is the ONLY command that runs across CLIs
type App interface {
	GetAuthClient() auth.Service
	GetKeychain() keychain.Keychain
	GetAuthURL() string
}
