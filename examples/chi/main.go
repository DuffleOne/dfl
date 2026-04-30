// Example program demonstrating dflhttp on a go-chi/chi router. The library
// itself has no awareness of chi: this file plugs chi into dflhttp.Router
// via a small adapter, then mounts the same handlers used in ./examples/std.
//
// Run:
//
//	go run ./examples/chi
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/duffleone/dfl/examples/api"
	dflhttp "github.com/duffleone/dfl/http"
	"github.com/go-chi/chi/v5"
)

// chiRouter implements dflhttp.Router on top of a *chi.Mux. Keeps the chi
// import out of the library; the cost is this adapter at the call site.
type chiRouter struct {
	mux        *chi.Mux
	coercer    dflhttp.Coercer
	prefix     string
	middleware []dflhttp.Middleware
}

var (
	_ dflhttp.Router = (*chiRouter)(nil)
	_ http.Handler   = (*chiRouter)(nil)
)

func newChiRouter() *chiRouter {
	return &chiRouter{
		mux:     chi.NewRouter(),
		coercer: dflhttp.DefaultCoercer,
	}
}

func (r *chiRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (r *chiRouter) Handle(method, path string, h dflhttp.HandlerFunc, mw ...dflhttp.Middleware) {
	fullPath := r.prefix + path
	chain := combineChain(r.middleware, mw)
	wrapped := applyMiddleware(h, chain)
	coercer := r.coercer

	r.mux.MethodFunc(method, fullPath, func(w http.ResponseWriter, req *http.Request) {
		copyPathValues(req)

		if err := wrapped(w, req); err != nil {
			writeError(w, err, coercer)
		}
	})
}

func (r *chiRouter) Group(prefix string) dflhttp.Router {
	return &chiRouter{
		mux:        r.mux,
		coercer:    r.coercer,
		prefix:     r.prefix + prefix,
		middleware: append([]dflhttp.Middleware(nil), r.middleware...),
	}
}

func (r *chiRouter) Use(mw ...dflhttp.Middleware) {
	r.middleware = append(r.middleware, mw...)
}

// copyPathValues mirrors chi's URL params onto the request via SetPathValue
// so the dflhttp binder's r.PathValue lookups work the same way they do
// under stdlib ServeMux.
func copyPathValues(r *http.Request) {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return
	}

	for i, key := range rctx.URLParams.Keys {
		r.SetPathValue(key, rctx.URLParams.Values[i])
	}
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

func main() {
	r := newChiRouter()

	rg := r.Group("/api")

	api.Health{
		GitCommitSHA: "deadbeef",
		Version:      "0.1.0",
	}.Mount(rg)

	api.NewUsers().Mount(rg)

	addr := ":8080"

	log.Printf("listening on %s (chi backend)", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
