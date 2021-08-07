package rpc

import (
	"context"
	_ "embed"

	authlib "dfl/lib/auth"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed list_u2f_keys.json
var listU2FKeysJSON string
var listU2FKeysSchema = gojsonschema.NewStringLoader(listU2FKeysJSON)

func (r *RPC) ListU2FKeys(ctx context.Context, req *auth.ListU2FKeysRequest) ([]*auth.PublicU2FKey, error) {
	authUser := authlib.GetUserContext(ctx)

	if authUser.ID != req.UserID && !authUser.Can("auth:list_keys") {
		return nil, cher.New(cher.AccessDenied, nil)
	}

	return r.app.ListU2FKeys(ctx, req)
}
