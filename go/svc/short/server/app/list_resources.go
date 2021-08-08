package app

import (
	"context"

	"dfl/svc/short"
)

// ListResources returns a list of all resources for a user
func (a *App) ListResources(ctx context.Context, req *short.ListResourcesRequest) ([]*short.Resource, error) {
	return a.DB.Q.ListResources(ctx, req)
}
