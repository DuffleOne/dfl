package commands

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"

	"dfl/lib/cli"
	"dfl/lib/keychain"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/dvsekhvalnov/jose2go/base64url"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tjarratt/babble"
)

func Login(clientID, scopes string, kc keychain.Keychain) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "login",
		Aliases: []string{"l"},
		Short:   "Login",

		RunE: func(cmd *cobra.Command, args []string) error {
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

			rootURL := viper.GetString("AUTH_URL")

			url := fmt.Sprintf("%s/authorize?%s", rootURL, params.Encode())

			fmt.Printf("%s: %s", cli.Warning("Careful"), "Ensure the state matches: ")
			fmt.Println(cli.Success(state))

			err := openBrowser(url)
			if err != nil {
				fmt.Printf("%s: %s", cli.Warning("Warning"), "Cannot open your browser for you, type in the URL yourself.")
			}

			authToken, err := authTokenPrompt.Run()
			if err != nil {
				return err
			}

			client, err := newClient(nil)
			if err != nil {
				return err
			}

			res, err := client.Token(context.Background(), &auth.TokenRequest{
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

			if err := kc.UpsertItem("Auth", authBytes); err != nil {
				return err
			}

			fmt.Println(cli.Success("Logged in!"))

			return nil
		},
	}

	cmd.Flags().StringVarP(&scopes, "scopes", "s", scopes, "Scopes to request, space delimited")

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
