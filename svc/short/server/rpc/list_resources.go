package rpc

import (
	"context"
	_ "embed"

	authlib "dfl/lib/auth"
	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed list_resources.json
var listResourcesJSON string
var listResourcesSchema = gojsonschema.NewStringLoader(listResourcesJSON)

func (r *RPC) ListResources(ctx context.Context, req *short.ListResourcesRequest) ([]*short.Resource, error) {
	authUser := authlib.GetUserContext(ctx)

	if err := authorizeRequest(req, authUser); err != nil {
		return nil, err
	}

	return r.app.ListResources(ctx, req)
}

func authorizeRequest(req *short.ListResourcesRequest, u *authlib.AuthUser) error {
	switch {
	case u.Can("short:admin"):
		return nil
	case req.OwnerID != nil && *req.OwnerID == u.ID:
		return nil
	default:
		return cher.New(cher.AccessDenied, nil)
	}
}
