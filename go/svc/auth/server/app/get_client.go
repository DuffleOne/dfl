package app

import (
	"context"

	"dfl/svc/auth"
)

func (a *App) GetClient(ctx context.Context, req *auth.GetClientRequest) (*auth.GetClientResponse, error) {
	client, err := a.FindClient(ctx, req.ClientID)
	if err != nil {
		return nil, err
	}

	return &auth.GetClientResponse{
		ID:   client.ID,
		Name: client.Name,
	}, nil
}
