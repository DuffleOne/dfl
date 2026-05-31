// Command pull is an events worker over Google Cloud Pub/Sub streaming pull. It
// publishes a UserCreated event and receives events from the subscription.
//
// Run (needs GCP credentials and existing topic/subscription):
//
//	PROJECT_ID=my-project go run .
package main

import (
	"context"
	"log"
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

	sink := gcppubsub.NewPullSink(ctx, client, "welcome-service")
	bus := events.NewBus(sink)

	bus.On(func(_ context.Context, e UserCreated) error {
		log.Printf("welcome %s", e.Email)

		return nil
	})

	if err := bus.Emit(ctx, UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		log.Fatalf("emit: %v", err)
	}

	log.Println("receiving from Pub/Sub...")
	select {} // Subscribe started the receiver; block so it keeps running
}
