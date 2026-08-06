package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// Adapt wires a typed handler into a HandlerFunc without registering it:
// parse Req via parser (DefaultRequestParser when nil), call the handler,
// then JSON-encode Resp, or write 204 for Empty. Errors return to the
// caller unwritten, joining the usual error pipeline, and an unbindable
// Req fails here, not on the first request. Router.Handle is Adapt plus
// registration; use Adapt when the dispatch isn't a route, as http/version does.
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
