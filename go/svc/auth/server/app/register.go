package app

import (
	"context"
	"time"

	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	"github.com/duo-labs/webauthn/webauthn"
)

func (a *App) Register(ctx context.Context, req *auth.RegisterConfirmRequest, credential *webauthn.Credential) (*auth.User, error) {
	invitation, err := a.CheckRegistrationValidity(ctx, req.Username, req.InviteCode)
	if err != nil {
		return nil, err
	}

	userID := ksuid.Generate("user").String()

	user, err := a.DB.RegisterUser(ctx, userID, invitation.ID, req.ChallengeID, req.Username, invitation.Scopes, req.KeyName, credential)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (a *App) CheckRegistrationValidity(ctx context.Context, username, inviteCode string) (*auth.Invitation, error) {
	if err := a.DB.Q.SearchByUsername(ctx, username); err != nil {
		return nil, err
	}

	invitation, err := a.DB.Q.GetInvitationByCode(ctx, inviteCode)
	if err != nil {
		return nil, err
	}

	if invitation.RedeemedAt != nil {
		return nil, cher.New("invitation_used", nil)
	}

	if invitation.ExpiresAt != nil && time.Now().After(*invitation.ExpiresAt) {
		return nil, cher.New("invitation_expired", nil)
	}

	return invitation, nil
}
