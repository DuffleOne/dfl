package storageproviders

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

type XKCDFS struct{}

func (x *XKCDFS) GenerateKey(string) string {
	panic("not implemented")
}

func (x *XKCDFS) SupportsSignedURLs() bool {
	return false
}

// GetSize of the XKCD image via a HEAD request to it's hosted content
func (x *XKCDFS) GetSize(ctx context.Context, r *short.Resource) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, r.Link, nil)
	if err != nil {
		return 0, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return 0, cher.New("cannot_make_request", cher.M{"status_code": res.StatusCode})
	}

	return strconv.Atoi(res.Header.Get("Content-Length"))
}

func (x *XKCDFS) Get(ctx context.Context, r *short.Resource) ([]byte, *time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.Link, nil)
	if err != nil {
		return nil, nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, nil, cher.New("cannot_make_request", cher.M{"status_code": res.StatusCode})
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, nil, err
	}

	modTime := res.Header.Get("Last-Modified")

	time, err := time.Parse(time.RFC1123, modTime)
	if err != nil {
		return nil, nil, err
	}

	return bytes, &time, nil
}

func (x *XKCDFS) PrepareUpload(ctx context.Context, key, contentType string, expiry time.Duration) (string, error) {
	panic("not implemented")
}

func (x *XKCDFS) Upload(ctx context.Context, key, contentType string, file bytes.Buffer) error {
	panic("not implemented")
}
