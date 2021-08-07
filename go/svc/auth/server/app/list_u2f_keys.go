package app

import (
	"context"

	"dfl/svc/auth"

	"github.com/dvsekhvalnov/jose2go/base64url"
)

func (a *App) ListU2FKeys(ctx context.Context, req *auth.ListU2FKeysRequest) ([]*auth.PublicU2FKey, error) {
	keys, err := a.DB.Q.ListU2FCredentials(ctx, req.UserID, req.IncludeUnsigned)
	if err != nil {
		return nil, err
	}

	credentials := []*auth.PublicU2FKey{}

	for _, key := range keys {
		credentials = append(credentials, &auth.PublicU2FKey{
			ID:        key.ID,
			Name:      key.Name,
			SignedAt:  key.SignedAt,
			PublicKey: base64url.Encode(key.Credential.PublicKey),
		})
	}

	return credentials, nil
}
