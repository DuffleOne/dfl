package http

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
)

// adapt produces a HandlerFunc from a typed handler. The handler shape is
// enforced by the compiler via generics; reflection is used only at
// registration time to walk the Req struct's tags.
//
// Req can be a struct, a pointer to a struct, Empty, or *Empty. For pointer
// Req with bindings, adapt allocates a fresh value on each request and the
// binder writes into it. For Empty/*Empty, no allocation happens; pointer
// reqs are passed as nil.
func adapt[Req, Resp any](handler func(context.Context, Req) (Resp, error)) (HandlerFunc, error) {
	reqType := reflect.TypeFor[Req]()
	bindType := reqType
	reqIsPtr := false

	if reqType.Kind() == reflect.Pointer {
		bindType = reqType.Elem()
		reqIsPtr = true
	}

	b, err := buildBinder(bindType)
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
		var (
			req        Req
			bindTarget any
		)

		switch {
		case reqIsPtr && b.noop:
			// *Empty (or *Empty alias): hand the handler nil; nothing to bind.
		case reqIsPtr:
			ptr := reflect.New(bindType)
			req = ptr.Interface().(Req)
			bindTarget = req
		default:
			bindTarget = &req
		}

		if bindTarget != nil {
			if err := b.bind(r, bindTarget); err != nil {
				return err
			}
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
