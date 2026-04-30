// Package http provides a thin abstraction over an http server, with
// typed handlers, structured errors, and a pluggable backend.
//
// A handler is a function of shape func(context.Context, Req) (Resp, error).
// The Router binds Req from path, query, and JSON body, calls the handler,
// then JSON-encodes Resp on success or runs error through a Coercer on
// failure.
//
// Use Empty as Req or Resp when there's no input or output of substance.
//
// The package is named http to keep import paths clean. To avoid the clash
// with stdlib net/http at use sites, alias on import:
//
//	import dflhttp "github.com/duffleone/dfl/http"
package http

import (
	"context"
	"net/http"
)

// M is a key/value bag used for structured metadata, notably on ReqError.
type M map[string]any

// Keys returns the keys of m in unspecified order.
func (m M) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}

// HandlerFunc is the lower-level handler shape, used by middleware and as the
// internal representation of a typed handler after binding has been wired up.
// Returning a non-nil error short-circuits the chain; the error is then run
// through the configured Coercer.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// Middleware wraps a HandlerFunc and returns a wrapped version. It can run
// code before or after, modify the request or response, short-circuit by
// returning an error without calling next, or transform the error returned
// by next.
type Middleware func(next HandlerFunc) HandlerFunc

// Coercer turns any error into a *ReqError suitable for serialising. nil in,
// nil out. Pluggable so callers can teach the router about their own error
// hierarchy (samber/oops, validation libs, etc.).
type Coercer func(error) *ReqError

// Router registers typed handlers. Each backend (stdlib, gin, etc.) provides
// a concrete implementation; the public surface is the same.
type Router interface {
	// Handle registers handler at method+path. handler must be a function of
	// shape func(context.Context, Req) (Resp, error); validation happens at
	// registration time and a mismatch panics. Per-route middleware in mw
	// runs inside any group middleware already on this router.
	Handle(method, path string, handler any, mw ...Middleware)

	// Group returns a sub-router whose routes are prefixed with prefix and
	// inherit the parent's middleware.
	Group(prefix string) Router

	// Use appends middleware to this router, applied to all routes registered
	// on this router (and its descendants) after the Use call.
	Use(mw ...Middleware)
}

// Handle is a typed convenience wrapper over Router.Handle that gives you
// compile-time checking of the handler shape. Equivalent to calling
// r.Handle(method, path, handler, mw...) directly.
func Handle[Req, Resp any](r Router, method, path string, handler func(context.Context, Req) (Resp, error), mw ...Middleware) {
	r.Handle(method, path, handler, mw...)
}
