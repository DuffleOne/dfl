package app

import (
	"context"

	"github.com/duo-labs/webauthn/webauthn"
)

func (a *App) CreateU2FChallenge(ctx context.Context, session *webauthn.SessionData) (string, error) {
	return a.DB.Q.CreateU2FChallenge(ctx, session)
}
