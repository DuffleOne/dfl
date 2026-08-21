// Example program: the same API on a go-chi/chi router. dflhttp has no
// awareness of chi; *chi.Mux simply satisfies MethodMux.
//
// Run:
//
//	go run ./http/examples/chi
package main

import (
	"log"
	"net/http"

	dflhttp "github.com/duffleone/dfl/http"
	"github.com/duffleone/dfl/http/examples/api"
	"github.com/duffleone/dfl/http/oops"
	"github.com/go-chi/chi/v5"
)

func main() {
	// chi has dedicated hooks for both fallbacks, so 404 and 405 come out
	// in dfl's error shape like everything else.
	mux := chi.NewMux()
	mux.NotFound(dflhttp.NotFoundHandler())
	mux.MethodNotAllowed(dflhttp.MethodNotAllowedHandler())

	r := dflhttp.NewRouter(mux, dflhttp.WithCoercer(oops.Coerce))
	r.Use(dflhttp.Recoverer())

	rg := r.Group("/api")

	api.Health{
		GitCommitSHA: "deadbeef",
		Version:      "0.1.0",
	}.Mount(rg)

	api.NewUsers().Mount(rg)
	api.NewWidgets().Mount(rg)

	addr := ":8080"

	log.Printf("listening on %s (chi backend)", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
