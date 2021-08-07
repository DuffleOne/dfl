package app

import (
	"context"

	"dfl/svc/short"
)

func (a *App) ShortenURL(ctx context.Context, url string) (*short.CreateResourceResponse, error) {
	return a.Client.ShortenURL(ctx, &short.CreateURLRequest{
		URL: url,
	})
}
