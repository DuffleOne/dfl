package rpc

import (
	"net/http"

	authlib "dfl/lib/auth"
	"dfl/lib/cher"
	"dfl/lib/rpc"
	"dfl/svc/short"
	"dfl/svc/short/server/app"

	"github.com/xeipuuv/gojsonschema"
)

var listResourcesSchema = gojsonschema.NewStringLoader(`{
	"type": "object",
	"additionalProperties": false,

	"required": [
		"include_deleted"
	],

	"properties": {
		"include_deleted": {
			"type": "boolean"
		},

		"username": {
			"type": "string",
			"minLength": 1
		},

		"limit": {
			"type": "number",
			"minimum": 1,
			"maximum": 100
		},

		"filter_mime": {
			"type": "string",
			"minLength": 1
		}
	}
}`)

func ListResources(a *app.App) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		err := rpc.ValidateRequest(r, listResourcesSchema)
		if err != nil {
			rpc.HandleError(w, r, err)
			return
		}

		req := &short.ListResourcesRequest{}
		err = rpc.ParseBody(r, req)
		if err != nil {
			rpc.HandleError(w, r, err)
			return
		}

		authUser := ctx.Value(authlib.UserContextKey).(authlib.AuthUser)
		if !authUser.Can("short:upload") && !authUser.Can("short:admin") {
			rpc.HandleError(w, r, cher.New(cher.AccessDenied, nil))
			return
		}

		if err = authorizeRequest(req, authUser); err != nil {
			rpc.HandleError(w, r, err)
			return
		}

		resources, err := a.ListResources(ctx, req)
		if err != nil {
			rpc.HandleError(w, r, err)
			return
		}

		rpc.WriteOut(w, r, resources)
	}
}

func authorizeRequest(req *short.ListResourcesRequest, u authlib.AuthUser) error {
	switch {
	case u.Can("short:admin"):
		return nil
	case req.Username != nil && *req.Username == u.Username:
		return nil
	default:
		return cher.New(cher.AccessDenied, nil)
	}
}
