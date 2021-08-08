package db

import (
	"context"
	"time"

	"dfl/svc/auth"

	"github.com/duo-labs/webauthn/webauthn"
)

func (db *DB) RegisterUser(ctx context.Context, userID, invitationID, challengeID, username, scopes string, keyName *string, credential *webauthn.Credential) (user *auth.User, err error) {
	now := time.Now()

	if err := db.DoTx(ctx, func(qw *QueryableWrapper) error {
		if user, err = qw.CreateUser(ctx, userID, username, scopes); err != nil {
			return err
		}

		if _, err := qw.CreateU2FCredential(ctx, userID, challengeID, keyName, credential, &now); err != nil {
			return err
		}

		return qw.RedeemInvite(ctx, invitationID, user.ID)
	}); err != nil {
		return nil, err
	}

	return
}
