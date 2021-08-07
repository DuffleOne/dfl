package app

import (
	"context"

	"dfl/svc/short"
)

// GetResourceByHash returns a resource when given a hash
func (a *App) GetResourceByHash(ctx context.Context, hash string) (*short.Resource, error) {
	return a.DB.Q.FindResourceByHash(ctx, hash)
}
