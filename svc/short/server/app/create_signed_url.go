package app

import (
	"context"
	"fmt"
	"time"

	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	pkgerr "github.com/pkg/errors"
)

// CreateSignedURL creates a file resource, but instead of accepting the file
// it generates a signed URL
func (a *App) CreateSignedURL(ctx context.Context, ownerID string, name *string, contentType string) (*short.CreateSignedURLResponse, error) {
	if !a.SP.SupportsSignedURLs() {
		return nil, cher.New("signed_urls_unsupported", nil)
	}

	fileID := ksuid.Generate("file").String()
	fileKey := a.SP.GenerateKey(fileID)

	fileRes, err := a.DB.Q.NewPendingFile(ctx, fileID, fileKey, ownerID, name, contentType)
	if err != nil {
		return nil, err
	}

	url, err := a.SP.PrepareUpload(ctx, fileKey, contentType, 15*time.Minute)
	if err != nil {
		return nil, pkgerr.Wrap(err, "unable to create presigned url")
	}

	hash := a.makeHash(fileRes.Serial)
	fullURL := fmt.Sprintf("%s/%s", a.RootURL, hash)

	gctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go a.saveHash(gctx, cancel, fileRes.Serial, hash)

	return &short.CreateSignedURLResponse{
		ResourceID: fileRes.ID,
		Type:       fileRes.Type,
		Hash:       hash,
		Name:       name,
		URL:        fullURL,
		SignedLink: url,
	}, nil
}

func (a *App) saveHash(ctx context.Context, c context.CancelFunc, serial int, hash string) error {
	defer c()

	return a.DB.Q.SaveHash(ctx, serial, hash)
}
