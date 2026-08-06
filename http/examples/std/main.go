// Example program: a full API on the stdlib *http.ServeMux, covering
// path, query, and body binding, 404s, validation, and 204s.
//
// Run:
//
//	go run ./http/examples/std
package main

import (
	"log"
	"net/http"

	dflhttp "github.com/duffleone/dfl/http"
	"github.com/duffleone/dfl/http/examples/api"
)

func main() {
	r := dflhttp.NewRouter(http.NewServeMux())

	rg := r.Group("/api")

	api.Health{
		GitCommitSHA: "deadbeef",
		Version:      "0.1.0",
	}.Mount(rg)

	api.NewUsers().Mount(rg)
	api.NewWidgets().Mount(rg)

	addr := ":8080"

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
