package app

import (
	"context"

	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

// GetResource returns a resource by shortcut or hash. Regardless of it's deleted status
func (a *App) GetResource(ctx context.Context, qi *QueryInput) (res *short.Resource, err error) {
	switch qi.QueryType {
	case Name:
		return a.GetResourceByName(ctx, qi.Filename())
	case Shortcut:
		return a.GetResourceByShortcut(ctx, qi.Input)
	case Hash:
		return a.GetResourceByHash(ctx, qi.Input)
	default:
		return nil, cher.New("unknown_query_type", cher.M{
			"query_input": qi,
		})
	}
}
