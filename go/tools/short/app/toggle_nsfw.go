package app

import (
	"context"

	"dfl/svc/short"
)

func (a *App) ToggleNSFW(ctx context.Context, query string) (bool, error) {
	details, err := a.Client.ViewDetails(ctx, &short.IdentifyResource{
		Query: query,
	})
	if err != nil {
		return false, err
	}

	return !details.NSFW, a.Client.SetNSFW(ctx, &short.SetNSFWRequest{
		IdentifyResource: short.IdentifyResource{
			Query: query,
		},
		NSFW: !details.NSFW,
	})
}
