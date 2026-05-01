package http

import (
	"context"
	"encoding/json"
	"net/http"
)

// adapt wires a typed handler into a HandlerFunc the underlying mux can
// register. It looks up the Router's RequestParser (DefaultRequestParser
// if unset), gives the parser a chance to verify the Req shape at
// registration, and at request time delegates Req parsing to it before
// invoking the handler and writing the response.
//
// All reflection lives behind RequestParser. Adapt itself touches no
// reflect — Req and Resp are pure type parameters as far as it's
// concerned, only known to be either a *Empty (or Empty) for 204, or
// something else for JSON encoding.
func (r *Router) adapt[Req, Resp any](handler func(context.Context, Req) (Resp, error)) (HandlerFunc, error) {
	parser := r.requestParser
	if parser == nil {
		parser = DefaultRequestParser
	}

	if pre, ok := parser.(preparable); ok {
		if err := pre.PrepareFor[Req](); err != nil {
			return nil, err
		}
	}

	isEmptyResp := false

	var respZero Resp
	switch any(respZero).(type) {
	case Empty, *Empty:
		isEmptyResp = true
	}

	return func(w http.ResponseWriter, httpReq *http.Request) error {
		req, err := parser.Parse[Req](httpReq)
		if err != nil {
			return err
		}

		resp, err := handler(httpReq.Context(), req)
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
