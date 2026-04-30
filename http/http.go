// Package http provides a thin abstraction over an http server, with
// typed handlers and structured errors.
//
// A handler is a function of shape func(context.Context, Req) (Resp, error).
// The Router binds Req from path, query, and JSON body, calls the handler,
// then JSON-encodes Resp on success or runs error through a Coercer on
// failure.
//
// Use Empty as Req or Resp when there's no input or output of substance.
//
// The Router wraps a Mux. Both stdlib *http.ServeMux (Go 1.22+) and
// go-chi/chi *chi.Mux satisfy Mux out of the box, so the package itself
// has no awareness of any specific routing implementation: pass whichever
// one you want to NewRouter and it works the same.
//
// The package is named http to keep import paths clean. To avoid the clash
// with stdlib net/http at use sites, alias on import:
//
//	import dflhttp "github.com/duffleone/dfl/http"
package http

import (
	"context"
	"encoding/json"
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

// Mux is the minimum a routing implementation must satisfy: be an
// http.Handler so the Router can serve traffic. NewRouter further requires
// the mux to support either MethodMux- or PatternMux-style registration.
type Mux interface {
	http.Handler
}

// MethodMux is a router that registers handlers per (method, pattern), like
// go-chi/chi *chi.Mux.
type MethodMux interface {
	Mux
	MethodFunc(method, pattern string, handler http.HandlerFunc)
}

// PatternMux is a router that registers handlers under stdlib's
// "METHOD /path" pattern, like *http.ServeMux on Go 1.22+. The parameter
// shape mirrors stdlib's: an unnamed func type rather than http.HandlerFunc,
// so *http.ServeMux satisfies this directly.
type PatternMux interface {
	Mux
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Router is dflhttp's unified router. It wraps a Mux, tracks group prefix
// and middleware, and turns typed handler registrations into HandlerFuncs
// registered on the underlying mux. Construct with NewRouter, register via
// Handle (or the typed package-level Handle), then hand the Router to
// http.Server.
type Router struct {
	mux        Mux
	register   func(method, pattern string, h http.HandlerFunc)
	coercer    Coercer
	prefix     string
	middleware []Middleware
}

var _ http.Handler = (*Router)(nil)

// Option configures a Router.
type Option func(*Router)

// WithCoercer sets the Coercer used to project handler and middleware errors
// onto *ReqError. Defaults to DefaultCoercer.
func WithCoercer(c Coercer) Option {
	return func(r *Router) {
		r.coercer = c
	}
}

// NewRouter wraps mux in a Router. mux must satisfy MethodMux (chi-style)
// or PatternMux (stdlib-style); NewRouter picks the right registration
// shape and panics if neither matches.
func NewRouter(mux Mux, opts ...Option) *Router {
	var register func(method, pattern string, h http.HandlerFunc)

	switch m := mux.(type) {
	case MethodMux:
		register = m.MethodFunc
	case PatternMux:
		register = func(method, pattern string, h http.HandlerFunc) {
			m.HandleFunc(method+" "+pattern, h)
		}
	default:
		panic("dflhttp: mux must implement MethodFunc(method, pattern, h) or HandleFunc(pattern, h)")
	}

	r := &Router{
		mux:      mux,
		register: register,
		coercer:  DefaultCoercer,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// Handle registers h at method+path. To register a typed handler use the
// package-level Handle, which adapts the handler down to a HandlerFunc.
func (r *Router) Handle(method, path string, h HandlerFunc, mw ...Middleware) {
	fullPath := r.prefix + path
	chain := combineChain(r.middleware, mw)
	wrapped := applyMiddleware(h, chain)
	coercer := r.coercer

	r.register(method, fullPath, func(w http.ResponseWriter, req *http.Request) {
		if err := wrapped(w, req); err != nil {
			writeError(w, err, coercer)
		}
	})
}

// Group returns a sub-router whose routes are prefixed with prefix and that
// inherits the parent's middleware as a snapshot at Group time. Middleware
// added to the parent after Group does not propagate to the returned group.
func (r *Router) Group(prefix string) *Router {
	return &Router{
		mux:        r.mux,
		register:   r.register,
		coercer:    r.coercer,
		prefix:     r.prefix + prefix,
		middleware: append([]Middleware(nil), r.middleware...),
	}
}

// Use appends middleware to this router. It applies to routes registered on
// this router after the Use call, and to descendants Group'd after.
func (r *Router) Use(mw ...Middleware) {
	r.middleware = append(r.middleware, mw...)
}

// Handle registers a typed handler at method+path on r. The handler shape
// is checked at compile time; bind setup walks the Req struct's tags once
// at registration and panics on a malformed Req.
func Handle[Req, Resp any](r *Router, method, path string, handler func(context.Context, Req) (Resp, error), mw ...Middleware) {
	h, err := adapt(handler)
	if err != nil {
		panic("dflhttp: " + err.Error())
	}

	r.Handle(method, path, h, mw...)
}

func combineChain(group, perRoute []Middleware) []Middleware {
	if len(group) == 0 {
		return perRoute
	}

	chain := make([]Middleware, 0, len(group)+len(perRoute))
	chain = append(chain, group...)
	chain = append(chain, perRoute...)

	return chain
}

func applyMiddleware(h HandlerFunc, mw []Middleware) HandlerFunc {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}

	return h
}

func writeError(w http.ResponseWriter, err error, coercer Coercer) {
	reqErr := coercer(err)
	if reqErr == nil {
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(reqErr.StatusCode)
	_ = json.NewEncoder(w).Encode(reqErr)
}
