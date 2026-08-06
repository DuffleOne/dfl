// Example app: the same handlers wired two ways, in-process (async) and
// over HTTP; POST /events/user.created and /events/orders-shipped.
//
// Run:
//
//	go run ./events/examples/app
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
