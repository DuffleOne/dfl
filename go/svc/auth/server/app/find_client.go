package app

import (
	"context"

	"dfl/svc/auth"
)

func (a *App) FindClient(ctx context.Context, id string) (*auth.Client, error) {
	return a.DB.Q.FindClient(ctx, id)
}
