// Example program demonstrating dflhttp on a go-chi/chi router. The dflhttp
// package itself has no awareness of chi: NewRouter accepts any mux that
// satisfies MethodMux or PatternMux, and *chi.Mux satisfies MethodMux out
// of the box.
//
// Run:
//
//	go run ./examples/chi
package main

import (
	"log"
	"net/http"

	"github.com/duffleone/dfl/examples/api"
	dflhttp "github.com/duffleone/dfl/http"
	"github.com/go-chi/chi/v5"
)

func main() {
	r := dflhttp.NewRouter(chi.NewMux())

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
