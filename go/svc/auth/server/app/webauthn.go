package app

import (
	"context"
	"dfl/svc/auth"
	"strings"

	"github.com/duo-labs/webauthn/webauthn"
)

func (a *App) ConvertUserForWA(ctx context.Context, user *auth.User, includeUnsigned bool) (*WAUser, error) {
	credentials, err := a.DB.Q.ListU2FCredentials(ctx, user.ID, includeUnsigned)
	if err != nil {
		return nil, err
	}

	var creds []webauthn.Credential

	for _, c := range credentials {
		creds = append(creds, c.Credential)
	}

	return &WAUser{
		ID:          user.ID,
		Name:        user.Username,
		Credentials: creds,
	}, nil
}

type WAUser struct {
	ID          string
	Name        string
	Credentials []webauthn.Credential
}

func (u WAUser) WebAuthnID() []byte {
	return []byte(u.ID)
}

func (u WAUser) WebAuthnName() string {
	name := strings.Replace(u.Name, " ", "_", -1)
	name = strings.ToLower(name)

	return name
}

func (u WAUser) WebAuthnDisplayName() string {
	return u.Name
}

func (u WAUser) WebAuthnIcon() string {
	return ""
}

func (u WAUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}
