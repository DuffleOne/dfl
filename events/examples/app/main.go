// Example app: the same handlers wired two ways, in-process (async) and over
// HTTP (POST /events/{name}).
//
// Run:
//
//	go run ./events/examples/app
//
// then in another shell:
//
//	curl -i -X POST localhost:8080/events/user.created   -d '{"id":"1","email":"a@b.com"}'
//	curl -i -X POST localhost:8080/events/orders-shipped -d '{"order_id":"7","carrier":"dhl"}'
//	curl -i -X POST localhost:8080/events/user.created   -d '{"id":"2","email":""}'  # 400
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/duffleone/dfl/events"
	dflhttp "github.com/duffleone/dfl/http"
)

func main() {
	bus := events.NewBus(events.NewMemSink())

	h := handlers{}
	h.Subscribe(bus)

	// In-process emit fans out to welcome + audit, asynchronously.
	if err := bus.Emit(context.Background(), UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		log.Fatalf("emit: %v", err)
	}

	r := dflhttp.NewRouter(http.NewServeMux())
	h.MountHTTP(bus, r)

	addr := ":8080"
	log.Printf("listening on %s, POST events to /events/{name}", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
