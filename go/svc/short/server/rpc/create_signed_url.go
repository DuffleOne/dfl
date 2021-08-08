package rpc

import (
	"context"
	_ "embed"

	authlib "dfl/lib/auth"
	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed create_signed_url.json
var createSignedURLJSON string
var createSignedURLSchema = gojsonschema.NewStringLoader(createSignedURLJSON)

// CreateSignedURL creates a signed URL for file uploads
func (r *RPC) CreateSignedURL(ctx context.Context, req *short.CreateSignedURLRequest) (*short.CreateSignedURLResponse, error) {
	authUser := authlib.GetUserContext(ctx)
	if !authUser.Can("short:upload") {
		return nil, cher.New(cher.AccessDenied, nil)
	}

	return r.app.CreateSignedURL(ctx, authUser.ID, req.Name, req.ContentType)
}
