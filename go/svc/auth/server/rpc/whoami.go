package rpc

import (
	"context"

	authlib "dfl/lib/auth"
	"dfl/svc/auth"
)

func (r *RPC) WhoAmI(ctx context.Context) (*auth.WhoAmIResponse, error) {
	authUser := authlib.GetUserContext(ctx)

	user, err := r.app.FindUser(ctx, authUser.ID)
	if err != nil {
		return nil, err
	}

	return &auth.WhoAmIResponse{
		UserID:   user.ID,
		Username: user.Username,
	}, nil
}
