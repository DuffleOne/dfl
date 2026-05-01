package http

import (
	"context"
	"encoding/json"
	"net/http"
)

// adapt produces a HandlerFunc from a typed handler. The handler shape is
// enforced by the compiler via generics; reflection is confined to the
// binder, which uses it solely to walk the Req struct's tags.
//
// Both Req and Resp are pointers (*ReqT and *RespT, including *Empty for
// routes with no input or no output). On every call, the handler receives
// a freshly-allocated, bound *ReqT (or nil for *Empty), and returns either
// a *RespT to JSON-encode or nil with an error.
func adapt[ReqT, RespT any](handler func(context.Context, *ReqT) (*RespT, error)) (HandlerFunc, error) {
	b, err := buildBinderFor[ReqT]()
	if err != nil {
		return nil, err
	}

	isEmptyResp := false

	var respZero *RespT
	if _, ok := any(respZero).(*Empty); ok {
		isEmptyResp = true
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		var (
			req    ReqT
			reqArg *ReqT
		)

		if !b.noop {
			if err := b.bind(r, &req); err != nil {
				return err
			}

			reqArg = &req
		}

		resp, err := handler(r.Context(), reqArg)
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
