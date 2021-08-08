package app

import (
	"context"

	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

// DeleteResource deletes a resource
func (a *App) DeleteResource(ctx context.Context, resource *short.Resource) error {
	if resource == nil {
		return cher.New(cher.NotFound, nil)
	}

	if resource.DeletedAt != nil {
		return cher.New(cher.NotFound, nil)
	}

	return a.DB.Q.DeleteResource(ctx, resource.ID)
}
