// Command push publishes to Google Cloud Pub/Sub and receives via a push
// subscription mounted as an HTTP endpoint. The sink is an http.Handler, so it
// mounts on any mux.
//
// Run (needs GCP credentials, a topic, and a push subscription pointed at this
// server's POST /events/pubsub):
//
//	PROJECT_ID=my-project go run .
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/pubsub"

	"github.com/duffleone/dfl/events"
	gcppubsub "github.com/duffleone/dfl/events/gcp/pubsub"
)

// UserCreated is the example event.
type UserCreated struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func main() {
	ctx := context.Background()

	client, err := pubsub.NewClient(ctx, os.Getenv("PROJECT_ID"))
	if err != nil {
		log.Fatalf("pubsub client: %v", err)
	}
	defer func() { _ = client.Close() }()

	sink := gcppubsub.NewPushSink(client)
	bus := events.NewBus(sink)

	bus.On(func(_ context.Context, e UserCreated) error {
		log.Printf("welcome %s", e.Email)

		return nil
	})

	// Publish one. Pub/Sub delivers it back to the endpoint below.
	if err := bus.Emit(ctx, UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		log.Fatalf("emit: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /events/pubsub", sink)

	log.Println("listening on :8080 for Pub/Sub push")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
