package rpc

import (
	"context"
	_ "embed"

	authlib "dfl/lib/auth"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed delete_key.json
var deleteKeyJSON string
var deleteKeySchema = gojsonschema.NewStringLoader(deleteKeyJSON)

func (r *RPC) DeleteKey(ctx context.Context, req *auth.DeleteKeyRequest) error {
	authUser := authlib.GetUserContext(ctx)

	if authUser.ID != req.UserID && !authUser.Can("auth:delete_keys") {
		return cher.New(cher.AccessDenied, nil)
	}

	return r.app.DeleteKey(ctx, req.UserID, req.KeyID)
}
