package cli

import (
	"github.com/atotto/clipboard"
	"github.com/gen2brain/beeep"
	log "github.com/sirupsen/logrus"
)

// AppName for notifications
const AppName = "DFL Short"

func Notify(title, body string) {
	err := beeep.Notify(title, body, "")
	if err != nil {
		log.Warn(err)
	}
}

func WriteClipboard(in string) {
	err := clipboard.WriteAll(in)
	if err != nil {
		log.Warn("Could not copy to clipboard.")
	}
}
