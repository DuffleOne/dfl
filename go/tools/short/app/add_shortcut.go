package app

import (
	"context"

	"dfl/svc/short"
)

func (a *App) AddShortcut(ctx context.Context, query, shortcut string) error {
	return a.Client.AddShortcut(ctx, &short.ChangeShortcutRequest{
		IdentifyResource: short.IdentifyResource{
			Query: query,
		},
		Shortcut: shortcut,
	})
}
