package app

import (
	"context"

	"dfl/svc/auth"
)

func (a *App) CreateInviteCode(ctx context.Context, userID string, req *auth.CreateInviteCodeRequest) (*auth.CreateInviteCodeResponse, error) {
	code, expiresAt, err := a.DB.Q.CreateInvitation(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	return &auth.CreateInviteCodeResponse{
		Code:      code,
		ExpiresAt: expiresAt,
	}, nil
}
