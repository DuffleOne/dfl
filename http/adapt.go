package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// Adapt wires a typed handler into a HandlerFunc without registering it
// anywhere: parse the request into Req via parser (DefaultRequestParser
// when nil), call the handler, then JSON-encode Resp on success, or write
// 204 when Resp is Empty. Binding, handler, and encoding errors return to
// the caller unwritten, so the result slots into the same error pipeline
// as a hand-rolled HandlerFunc.
//
// The Req shape is verified here, once: a Req the parser can't bind is an
// error at Adapt time, not on the first request. Router.Handle is Adapt
// plus registration; use Adapt directly when the dispatch decision isn't
// a route, the way http/version picks a handler variant per request.
//
// All reflection lives behind RequestParser. Adapt itself touches no
// reflect: Req and Resp are pure type parameters as far as it's concerned,
// only known to be either a *Empty (or Empty) for 204, or something else
// for JSON encoding.
func Adapt[Req, Resp any](parser *RequestParser, handler func(context.Context, Req) (Resp, error)) (HandlerFunc, error) {
	if parser == nil {
		parser = DefaultRequestParser
	}

	if err := parser.PrepareFor[Req](); err != nil {
		return nil, err
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

		// Encode to a buffer first, so a Resp that fails to encode surfaces
		// as a clean error response rather than a half-written 200.
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(resp); err != nil {
			return err
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = buf.WriteTo(w)

		return nil
	}, nil
}

// adapt adapts with the Router's configured RequestParser, keeping
// Router.Handle a thin wrapper over the exported entry point.
func (r *Router) adapt[Req, Resp any](handler func(context.Context, Req) (Resp, error)) (HandlerFunc, error) {
	return Adapt(r.requestParser, handler)
}
