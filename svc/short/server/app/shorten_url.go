package app

import (
	"context"
	"fmt"
	"time"

	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/ksuid"
)

// ShortenURL shortens a URL
func (a *App) ShortenURL(ctx context.Context, url, ownerID string) (*short.CreateResourceResponse, error) {
	urlID := ksuid.Generate("url").String()

	// save to DB
	urlRes, err := a.DB.Q.NewURL(ctx, urlID, ownerID, url)
	if err != nil {
		return nil, err
	}

	hash := a.makeHash(urlRes.Serial)
	fullURL := fmt.Sprintf("%s/%s", a.RootURL, hash)

	gctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go a.saveHash(gctx, cancel, urlRes.Serial, hash)

	return &short.CreateResourceResponse{
		ResourceID: urlRes.ID,
		Type:       urlRes.Type,
		Hash:       hash,
		URL:        fullURL,
	}, nil
}
