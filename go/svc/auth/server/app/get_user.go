package app

import (
	"context"

	"dfl/svc/auth"
)

func (a *App) GetUserByName(ctx context.Context, username string) (*auth.User, error) {
	return a.DB.Q.GetUserByName(ctx, username)
}

func (a *App) FindUser(ctx context.Context, id string) (*auth.User, error) {
	return a.DB.Q.FindUser(ctx, id)
}
