package app

import (
	"context"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

func (a *App) DeleteKey(ctx context.Context, userID, keyID string) error {
	credential, err := a.DB.Q.FindU2FCredential(ctx, keyID)
	if err != nil {
		return err
	}

	if credential.UserID != userID {
		return cher.New(cher.AccessDenied, nil)
	}

	signedKeys, err := a.ListU2FKeys(ctx, &auth.ListU2FKeysRequest{
		UserID:          userID,
		IncludeUnsigned: false,
	})
	if err != nil {
		return err
	}

	if len(signedKeys) == 1 && keyID == signedKeys[0].ID {
		return cher.New("last_signed_key", cher.M{"key_id": keyID})
	}

	return a.DB.Q.DeleteU2FCredential(ctx, userID, keyID)
}
