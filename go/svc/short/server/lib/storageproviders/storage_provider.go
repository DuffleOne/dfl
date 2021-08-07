package storageproviders

import (
	"bytes"
	"context"
	"time"

	"dfl/svc/short"
)

// StorageProvider is an interface all custom defined storage providers must conform to
type StorageProvider interface {
	GenerateKey(string) string
	SupportsSignedURLs() bool
	GetSize(context.Context, *short.Resource) (int, error)
	Get(context.Context, *short.Resource) ([]byte, *time.Time, error)
	PrepareUpload(ctx context.Context, key, contentType string, expiry time.Duration) (string, error)
	Upload(ctx context.Context, key, contentType string, file bytes.Buffer) error
}
