package app

import (
	"context"
	"strings"

	"dfl/svc/short"
	"dfl/svc/short/server/db"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

var bannedPrefixes = []string{
	"xkcd",
}

func (a *App) AddShortcut(ctx context.Context, resource *short.Resource, shortcut string) error {
	err := a.DB.Q.FindShortcutConflicts(ctx, []string{shortcut})
	if err != nil {
		return cher.New("shortcuts_already_taken", cher.M{"shortcut": shortcut}, cher.Coerce(err))
	}

	for _, prefix := range bannedPrefixes {
		if strings.HasPrefix(shortcut, prefix) {
			return cher.New("invalid_shortcut", cher.M{"prefix": prefix})
		}
	}

	return a.DB.Q.ChangeShortcut(ctx, db.ArrayAdd, resource.ID, shortcut)
}
