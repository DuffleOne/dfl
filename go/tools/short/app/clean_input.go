package app

import (
	"fmt"
	"strings"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/manifoldco/promptui"
)

func (a *App) CleanInput(args []string) (string, error) {
	if len(args) == 1 {
		return a.Trim(args[0]), nil
	}

	query, err := queryPrompt.Run()
	if err != nil {
		return "", err
	}

	return a.Trim(query), nil
}

func (a *App) Trim(in string) string {
	return strings.TrimPrefix(in, fmt.Sprintf("%s/", a.RootURL))
}

var queryPrompt = promptui.Prompt{
	Label: "Query",
	Validate: func(input string) error {
		if len(input) >= 1 {
			return nil
		}

		return cher.New("missing_query", nil)
	},
}
