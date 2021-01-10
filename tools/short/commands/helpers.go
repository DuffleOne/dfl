package commands

import (
	"fmt"

	"dfl/lib/cli"
	"dfl/lib/keychain"
	"dfl/svc/short"

	"github.com/atotto/clipboard"
	b "github.com/gen2brain/beeep"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// AppName for notifications
const AppName = "DFL Short"

func notify(title, body string) {
	err := b.Notify(title, body, "")
	if err != nil {
		log.Warn(err)
	}
}

func makeClient(keychain keychain.Keychain) short.Service {
	return short.NewClient(rootURL(), cli.AuthHeader(keychain, "short"))
}

func rootURL() string {
	return fmt.Sprintf("%s/", viper.Get("SHORT_URL").(string))
}

func writeClipboard(in string) {
	err := clipboard.WriteAll(in)
	if err != nil {
		log.Warn("Could not copy to clipboard.")
	}
}
