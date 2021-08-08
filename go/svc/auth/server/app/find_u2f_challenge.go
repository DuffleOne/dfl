package app

import (
	"context"

	"github.com/duo-labs/webauthn/webauthn"
)

func (a *App) FindU2FChallenge(ctx context.Context, id string) (*webauthn.SessionData, error) {
	return a.DB.Q.FindU2FChallenge(ctx, id)
}
