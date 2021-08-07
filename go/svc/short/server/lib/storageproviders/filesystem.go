package storageproviders

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"dfl/svc/short"

	"golang.org/x/sync/errgroup"
)

// LocalFileSystem is a storage provider for the local filesystem
type LocalFileSystem struct {
	folder      string
	permissions os.FileMode
}

// NewLFSProviderFromCfg makes a new FileSystem provider from env vars
func NewLFSProviderFromCfg(folder string, permissions os.FileMode) (StorageProvider, error) {
	return &LocalFileSystem{
		folder:      folder,
		permissions: permissions,
	}, nil
}

// GenerateKey generates a file key used as a filename
func (fs *LocalFileSystem) GenerateKey(fileID string) string {
	return fmt.Sprintf("%s/%s", fs.folder, fileID)
}

// SupportsSignedURLs lets the service know if it can use prepared URLs
func (fs *LocalFileSystem) SupportsSignedURLs() bool {
	return false
}

// Get a resource from the storage provider
func (fs *LocalFileSystem) Get(ctx context.Context, resource *short.Resource) ([]byte, *time.Time, error) {
	g, _ := errgroup.WithContext(ctx)

	var bytes []byte
	var lastModified time.Time

	g.Go(func() (err error) {
		var fileInfo os.FileInfo

		fileInfo, err = os.Stat(resource.Link)
		if err != nil {
			return err
		}

		lastModified = fileInfo.ModTime()

		return
	})

	g.Go(func() (err error) {
		bytes, err = ioutil.ReadFile(resource.Link)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	return bytes, &lastModified, nil
}

// PrepareUpload prepares an upload into the storage provider
func (fs *LocalFileSystem) PrepareUpload(ctx context.Context, key, contentType string, expiry time.Duration) (string, error) {
	return "", errors.New("unsupported")
}

// Upload a resource into the storage provider
func (fs *LocalFileSystem) Upload(_ context.Context, key, contentType string, file bytes.Buffer) error {
	return ioutil.WriteFile(key, file.Bytes(), fs.permissions)
}

// GetSize returns the size of the byte content of a file
func (fs *LocalFileSystem) GetSize(_ context.Context, resource *short.Resource) (int, error) {
	fileInfo, err := os.Stat(resource.Link)
	if err != nil {
		return 0, err
	}

	size := fileInfo.Size()

	return int(size), nil
}
