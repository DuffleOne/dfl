package app

import (
	"context"

	"dfl/svc/short"
	"dfl/svc/short/server/db"
)

func (a *App) RemoveShortcut(ctx context.Context, resource *short.Resource, shortcut string) error {
	return a.DB.Q.ChangeShortcut(ctx, db.ArrayRemove, resource.ID, shortcut)
}
