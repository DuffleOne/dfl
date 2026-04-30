// Example program demonstrating dflhttp on the stdlib *http.ServeMux.
//
// Endpoints:
//
//	GET  /api/health           empty req,  status resp
//	GET  /api/sha              empty req,  status resp
//	GET  /api/version          empty req,  status resp
//	POST /api/ping             empty req,  empty resp (204)
//	GET  /api/users            query req,  list resp
//	GET  /api/users/{id}       path req,   struct resp (404 on miss)
//	POST /api/users            body req,   struct resp
//	PUT  /api/users/{id}       path+body,  struct resp
//	POST /api/widgets/{id}     path+query+body, validated; 400 on failure
//
// Run:
//
//	go run ./examples/std
package main

import (
	"log"
	"net/http"

	"github.com/duffleone/dfl/examples/api"
	dflhttp "github.com/duffleone/dfl/http"
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
