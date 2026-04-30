// Example program demonstrating dflhttp.
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
//	go run ./examples
//
// Then:
//
//	curl localhost:8080/api/health
//	curl -X POST localhost:8080/api/ping -i
//	curl 'localhost:8080/api/users?limit=5'
//	curl localhost:8080/api/users/1
//	curl -X POST localhost:8080/api/users -H 'content-type: application/json' -d '{"name":"alice"}'
//	curl -X PUT  localhost:8080/api/users/1 -H 'content-type: application/json' -d '{"name":"alicia"}'
package main

import (
	"log"
	"net/http"

	dflstd "github.com/duffleone/dfl/http/std"
)

func main() {
	r := dflstd.New()

	api := r.Group("/api")

	Health{
		GitCommitSHA: "deadbeef",
		Version:      "0.1.0",
	}.Mount(api)

	NewUsers().Mount(api)

	addr := ":8080"

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
