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
func adapt[Req, Resp any](handler func(context.Context, Req) (Resp, error)) (HandlerFunc, error) {
	b, err := buildBinder(reflect.TypeFor[Req]())
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
