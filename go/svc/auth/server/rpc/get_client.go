package rpc

import (
	"context"
	_ "embed"

	"dfl/svc/auth"

	"github.com/xeipuuv/gojsonschema"
)

//go:embed get_client.json
var getClientJSON string
var getClientSchema = gojsonschema.NewStringLoader(getClientJSON)

func (r *RPC) GetClient(ctx context.Context, req *auth.GetClientRequest) (*auth.GetClientResponse, error) {
	return r.app.GetClient(ctx, req)
}
