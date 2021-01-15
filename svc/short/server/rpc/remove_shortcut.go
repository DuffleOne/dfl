package rpc

import (
	"net/http"

	authlib "dfl/lib/auth"
	"dfl/lib/rpc"
	"dfl/svc/short"
	"dfl/svc/short/server/app"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/xeipuuv/gojsonschema"
)

var removeShortcutSchema = gojsonschema.NewStringLoader(`{
	"type": "object",
	"additionalProperties": false,

	"required": [
		"query",
		"shortcut"
	],

	"properties": {
		"query": {
			"type": "string",
			"minLength": 3
		},

		"shortcut": {
			"type": "string",
			"minLength": 3
		}
	}
}`)

func RemoveShortcut(a *app.App, w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	err := rpc.ValidateRequest(r, removeShortcutSchema)
	if err != nil {
		return err
	}

	req := &short.ChangeShortcutRequest{}
	err = rpc.ParseBody(r, req)
	if err != nil {
		return err
	}

	authUser := ctx.Value(authlib.UserContextKey).(authlib.AuthUser)
	if !authUser.Can("short:upload") && !authUser.Can("short:admin") {
		return cher.New(cher.AccessDenied, nil)
	}

	qi := a.ParseQueryType(req.Query)

	if len(qi) != 1 {
		return cher.New("multi_query_not_supported", cher.M{"query": qi})
	}

	if qi[0].QueryType == app.Name {
		return cher.New("cannot_query_resource_by_name", cher.M{"query": qi})
	}

	resource, err := a.GetResource(ctx, qi[0])
	if err != nil {
		return err
	}

	if resource.Owner != authUser.Username {
		return cher.New(cher.AccessDenied, nil)
	}

	return a.RemoveShortcut(ctx, resource, req.Shortcut)
}
