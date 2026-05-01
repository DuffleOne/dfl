package http

import (
	"context"
	"encoding/json"
	"net/http"
)

// adapt produces a HandlerFunc from a typed handler. Reflection is confined
// to the binder, which uses it solely to walk the Req struct's tags and
// (when Req is a pointer) to allocate the value on first bind. Adapt
// itself uses no reflection at all.
//
// Req and Resp can be any shape: struct, *struct, Empty, *Empty. The pointer
// convention is what handlers in this codebase use, but the framework
// doesn't insist on it.
func adapt[Req, Resp any](handler func(context.Context, Req) (Resp, error)) (HandlerFunc, error) {
	b, err := buildBinderFor[Req]()
	if err != nil {
		return nil, err
	}

	isEmptyResp := false

	var respZero Resp
	switch any(respZero).(type) {
	case Empty, *Empty:
		isEmptyResp = true
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		var req Req

		if err := b.bind(r, &req); err != nil {
			return err
		}

		resp, err := handler(r.Context(), req)
		if err != nil {
			return err
		}

		if isEmptyResp {
			w.WriteHeader(http.StatusNoContent)

			return nil
		}

		w.Header().Set("Content-Type", "application/json")

		return json.NewEncoder(w).Encode(resp)
	}, nil
}
