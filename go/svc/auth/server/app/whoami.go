package app

import (
	"context"

	"dfl/svc/auth"
)

func (a *App) WhoAmI(ctx context.Context, userID string) (*auth.WhoAmIResponse, error) {
	user, err := a.FindUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &auth.WhoAmIResponse{
		UserID:   user.ID,
		Username: user.Username,
	}, nil
}
