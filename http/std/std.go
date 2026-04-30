// Package std implements a dflhttp.Router backed by net/http (stdlib).
//
// Construct with New, register handlers via Mount methods or directly via
// Handle, then hand the Router to http.Server. Group adds a path prefix and
// inherits middleware as a snapshot at Group time.
package std

import (
	"encoding/json"
	"net/http"

	dflhttp "github.com/duffleone/dfl/http"
)

// Router is a dflhttp.Router backed by *http.ServeMux. It also implements
// http.Handler so you can hand it directly to http.Server.
type Router struct {
	mux        *http.ServeMux
	coercer    dflhttp.Coercer
	prefix     string
	middleware []dflhttp.Middleware
}

var (
	_ dflhttp.Router = (*Router)(nil)
	_ http.Handler   = (*Router)(nil)
)

// Option configures a Router.
type Option func(*Router)

// WithCoercer sets the Coercer used to project handler and middleware errors
// onto *dflhttp.ReqError. Defaults to dflhttp.DefaultCoercer.
func WithCoercer(c dflhttp.Coercer) Option {
	return func(r *Router) {
		r.coercer = c
	}
}

// New returns a Router with the given options.
func New(opts ...Option) *Router {
	r := &Router{
		mux:     http.NewServeMux(),
		coercer: dflhttp.DefaultCoercer,
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

// Handle registers h at method+path. To register a typed handler, use
// dflhttp.Handle, which adapts the handler down to a HandlerFunc.
func (r *Router) Handle(method, path string, h dflhttp.HandlerFunc, mw ...dflhttp.Middleware) {
	fullPath := r.prefix + path
	chain := combineChain(r.middleware, mw)
	wrapped := applyMiddleware(h, chain)
	coercer := r.coercer

	r.mux.HandleFunc(method+" "+fullPath, func(w http.ResponseWriter, req *http.Request) {
		if err := wrapped(w, req); err != nil {
			writeError(w, err, coercer)
		}
	})
}

// Group returns a sub-router whose routes are prefixed with prefix and that
// inherits the parent's middleware as a snapshot at Group time. Middleware
// added to the parent after Group does not propagate to the returned group.
func (r *Router) Group(prefix string) dflhttp.Router {
	return &Router{
		mux:        r.mux,
		coercer:    r.coercer,
		prefix:     r.prefix + prefix,
		middleware: append([]dflhttp.Middleware(nil), r.middleware...),
	}
}

// Use appends middleware to this router. It applies to routes registered on
// this router after the Use call, and to descendants Group'd after.
func (r *Router) Use(mw ...dflhttp.Middleware) {
	r.middleware = append(r.middleware, mw...)
}

func combineChain(group, perRoute []dflhttp.Middleware) []dflhttp.Middleware {
	if len(group) == 0 {
		return perRoute
	}

	chain := make([]dflhttp.Middleware, 0, len(group)+len(perRoute))
	chain = append(chain, group...)
	chain = append(chain, perRoute...)

	return chain
}

func applyMiddleware(h dflhttp.HandlerFunc, mw []dflhttp.Middleware) dflhttp.HandlerFunc {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}

	return h
}

func writeError(w http.ResponseWriter, err error, coercer dflhttp.Coercer) {
	reqErr := coercer(err)
	if reqErr == nil {
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(reqErr.StatusCode)
	_ = json.NewEncoder(w).Encode(reqErr)
}
