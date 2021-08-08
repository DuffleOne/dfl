package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dfl/lib/cache"
	"dfl/svc/short"
	"dfl/svc/short/server/lib/storageproviders"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

const maxFileSize = 128
const maxCacheSize = 32
const byteJump = 1024

// MaxCacheSize is the maximum size of a file for it to skip the cache: 536,870,912
const MaxCacheSize = byteJump * byteJump * maxCacheSize

// MaxFileSize is the maximum file size it will file
const MaxFileSize = byteJump * byteJump * maxFileSize

// GetFile returns a file from the cache,or the file provider
func (a *App) GetFile(ctx context.Context, resource *short.Resource) ([]byte, *time.Time, error) {
	cacheKey := fmt.Sprintf("file/%s", resource.Link)

	if item, found := a.Redis.Get(cacheKey); found {
		return item.Content, item.ModTime, nil
	}

	sp := a.SP

	if resource.Type == "xkcd" {
		sp = &storageproviders.XKCDFS{}
	}

	size, err := sp.GetSize(ctx, resource)
	if err != nil {
		return nil, nil, err
	}

	if size >= MaxFileSize {
		return nil, nil, cher.New("too_big", nil)
	}

	bytes, lastModified, err := sp.Get(ctx, resource)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") {
			return nil, nil, cher.New(cher.NotFound, nil)
		}

		return nil, nil, err
	}

	if len(bytes) < MaxCacheSize {
		a.Redis.Set(cacheKey, &cache.CacheItem{
			Content: bytes,
			ModTime: lastModified,
		})
	}

	return bytes, lastModified, nil
}
