package rpc

import (
	"context"
	_ "embed"

	authlib "dfl/lib/auth"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed create_client.json
var createClientJSON string
var createClientSchema = gojsonschema.NewStringLoader(createClientJSON)

func (r *RPC) CreateClient(ctx context.Context, req *auth.CreateClientRequest) (*auth.CreateClientResponse, error) {
	authUser := authlib.GetUserContext(ctx)

	if !authUser.Can("auth:create_client") {
		return nil, cher.New(cher.AccessDenied, nil)
	}

	return r.app.CreateClient(ctx, req)
}
