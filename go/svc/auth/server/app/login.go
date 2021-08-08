package app

import (
	"context"

	"dfl/svc/auth"
)

func (a *App) CheckLoginValidity(ctx context.Context, user *auth.User) error {
	return nil
}
