package app

import (
	"context"
)

func (a *App) SetNSFW(ctx context.Context, resourceID string, nsfw bool) error {
	return a.DB.Q.SetNSFW(ctx, resourceID, nsfw)
}
