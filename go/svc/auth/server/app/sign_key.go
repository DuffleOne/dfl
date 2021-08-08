package app

import (
	"context"

	"dfl/svc/auth"
)

func (a *App) SignKey(ctx context.Context, user *auth.User, keyToSign string) error {
	if err := a.CanSign(ctx, user.ID, keyToSign); err != nil {
		return err
	}

	return a.DB.Q.SignU2FCredential(ctx, keyToSign)
}
