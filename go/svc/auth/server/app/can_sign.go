package app

import (
	"context"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

func (a *App) CanSign(ctx context.Context, userID, keyToSign string) error {
	credential, err := a.DB.Q.FindU2FCredential(ctx, keyToSign)
	if err != nil {
		return err
	}

	if credential.DeletedAt != nil {
		return cher.New(cher.NotFound, nil)
	}

	if credential.UserID != userID {
		return cher.New(cher.NotFound, nil)
	}

	if credential.SignedAt != nil {
		return cher.New("already_signed", nil)
	}

	return nil
}
