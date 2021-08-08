package app

import (
	"context"

	"dfl/svc/short"
)

// GetResourceByName returns a resource from it's name
func (a *App) GetResourceByName(ctx context.Context, name string) (*short.Resource, error) {
	return a.DB.Q.FindResourceByName(ctx, name)
}
