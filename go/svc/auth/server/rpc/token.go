package rpc

import (
	"context"
	_ "embed"

	"dfl/svc/auth"

	"github.com/xeipuuv/gojsonschema"
)

//go:embed token.json
var tokenJSON string
var tokenSchema = gojsonschema.NewStringLoader(tokenJSON)

func (r *RPC) Token(ctx context.Context, req *auth.TokenRequest) (*auth.TokenResponse, error) {
	return r.app.Token(ctx, req)
}
