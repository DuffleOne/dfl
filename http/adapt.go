package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
)

// StatusCoder lets a Resp type pick its success status, the way Empty picks
// 204: implement it to return 201 for a create or 202 for an accepted job.
// It's consulted once at registration on a zero value, so return a
// constant. Without it, a body writes 200.
type StatusCoder interface {
	SuccessStatus() int
}

// successStatusFor resolves Resp's success status at registration. A nil
// pointer zero value is swapped for a pointer to a zero struct first, so a
// value-receiver SuccessStatus on a pointer Resp doesn't panic.
func successStatusFor[Resp any]() int {
	var zero Resp

	sc, ok := any(zero).(StatusCoder)
	if !ok {
		return http.StatusOK
	}

	if rv := reflect.ValueOf(zero); rv.Kind() == reflect.Pointer && rv.IsNil() {
		sc = reflect.New(rv.Type().Elem()).Interface().(StatusCoder)
	}

	return sc.SuccessStatus()
}

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

	successStatus := successStatusFor[Resp]()

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

		if successStatus != http.StatusOK {
			w.WriteHeader(successStatus)
		}

		_, _ = buf.WriteTo(w)

		return nil
	}, nil
}

// adapt adapts with the Router's configured RequestParser, keeping
// Router.Handle a thin wrapper over the exported entry point.
func (r *Router) adapt[Req, Resp any](handler func(context.Context, Req) (Resp, error)) (HandlerFunc, error) {
	return Adapt(r.requestParser, handler)
}
