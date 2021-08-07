package app

import (
	"context"
	"dfl/svc/short"
)

func (a *App) DeleteResource(ctx context.Context, query string) error {
	return a.Client.DeleteResource(ctx, &short.IdentifyResource{
		Query: query,
	})
}
