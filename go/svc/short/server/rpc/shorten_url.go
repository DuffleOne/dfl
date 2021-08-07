package rpc

import (
	"context"
	_ "embed"

	authlib "dfl/lib/auth"
	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed shorten_url.json
var shortenURLJSON string
var shortenURLSchema = gojsonschema.NewStringLoader(shortenURLJSON)

func (r *RPC) ShortenURL(ctx context.Context, req *short.CreateURLRequest) (*short.CreateResourceResponse, error) {
	authUser := authlib.GetUserContext(ctx)
	if !authUser.Can("short:upload") {
		return nil, cher.New(cher.AccessDenied, nil)
	}

	return r.app.ShortenURL(ctx, req.URL, authUser.ID)
}
