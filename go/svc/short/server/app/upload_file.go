package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"dfl/lib/cache"
	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/ksuid"
)

// UploadFile is an app method that takes in a file and stores it
func (a *App) UploadFile(ctx context.Context, ownerID string, req *short.CreateFileRequest) (*short.CreateResourceResponse, error) {
	// get user
	bytes := req.File.Bytes()
	contentType := http.DetectContentType(bytes)
	fileID := ksuid.Generate("file").String()
	fileKey := a.SP.GenerateKey(fileID)
	name := req.Name

	// upload to the file provider
	err := a.SP.Upload(ctx, fileKey, contentType, req.File)
	if err != nil {
		return nil, err
	}

	// save to DB
	fileRes, err := a.DB.Q.NewFile(ctx, fileID, fileKey, ownerID, name, contentType)
	if err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("file/%s", fileRes.Link)
	now := time.Now()

	if len(bytes) < MaxCacheSize {
		a.Redis.Set(cacheKey, &cache.CacheItem{
			Content: bytes,
			ModTime: &now,
		})
	}

	hash := a.makeHash(fileRes.Serial)
	fullURL := fmt.Sprintf("%s/%s", a.RootURL, hash)

	gctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go a.saveHash(gctx, cancel, fileRes.Serial, hash)

	return &short.CreateResourceResponse{
		ResourceID: fileRes.ID,
		Type:       fileRes.Type,
		Hash:       hash,
		URL:        fullURL,
	}, nil
}

func (a *App) makeHash(serial int) string {
	e, _ := a.Hasher.Encode([]int{serial})

	return e
}
