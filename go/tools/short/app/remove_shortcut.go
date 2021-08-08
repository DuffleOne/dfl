package app

import (
	"context"

	"dfl/svc/short"
)

func (a *App) RemoveShortcut(ctx context.Context, query, shortcut string) error {
	return a.Client.RemoveShortcut(ctx, &short.ChangeShortcutRequest{
		IdentifyResource: short.IdentifyResource{
			Query: query,
		},
		Shortcut: shortcut,
	})
}
