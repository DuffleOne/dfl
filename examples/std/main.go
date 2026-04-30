// Example program demonstrating dflhttp on the stdlib ServeMux backend.
//
// Endpoints:
//
//	GET  /api/health       empty req,  string resp
//	GET  /api/sha          empty req,  string resp
//	GET  /api/version      empty req,  string resp
//	POST /api/ping         empty req,  empty resp (204)
//	GET  /api/users        query req,  list resp
//	GET  /api/users/{id}   path req,   struct resp (404 on miss)
//	POST /api/users        body req,   struct resp
//	PUT  /api/users/{id}   path+body,  struct resp
//
// Run:
//
//	go run ./examples/std
package main

import (
	"log"
	"net/http"

	"github.com/duffleone/dfl/examples/api"
	dflstd "github.com/duffleone/dfl/http/std"
)

func main() {
	r := dflstd.New()

	rg := r.Group("/api")

	api.Health{
		GitCommitSHA: "deadbeef",
		Version:      "0.1.0",
	}.Mount(rg)

	api.NewUsers().Mount(rg)

	addr := ":8080"

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
