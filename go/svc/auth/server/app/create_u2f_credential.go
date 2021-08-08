package app

import (
	"context"
	"time"

	"github.com/duo-labs/webauthn/webauthn"
)

func (a *App) CreateU2FCredential(ctx context.Context, userID, challengeID string, keyName *string, credential *webauthn.Credential, signedAt *time.Time) (string, error) {
	return a.DB.Q.CreateU2FCredential(ctx, userID, challengeID, keyName, credential, signedAt)
}
