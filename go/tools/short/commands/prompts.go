package commands

import (
	"net/url"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/manifoldco/promptui"
)

var filePrompt = promptui.Prompt{
	Label: "File",
	Validate: func(input string) error {
		if len(input) >= 1 {
			return nil
		}

		return cher.New("missing_file", nil)
	},
}

var shortcutPrompt = promptui.Prompt{
	Label: "Shortcut",
	Validate: func(input string) error {
		if len(input) >= 1 {
			return nil
		}

		return cher.New("missing_shortcut", nil)
	},
}

var urlPrompt = promptui.Prompt{
	Label: "URL",
	Validate: func(input string) error {
		if len(input) == 0 {
			return cher.New("missing_url", nil)
		}

		u, err := url.ParseRequestURI(input)
		if err != nil {
			return err
		}

		if u.Scheme == "" || u.Host == "" {
			return cher.New("malformed_url", nil)
		}

		return nil
	},
}
