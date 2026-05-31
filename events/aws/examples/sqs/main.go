// Command sqs is an events worker over Amazon SQS. It publishes a UserCreated
// event and then receives events from the queue, dispatching to the handler.
//
// Run (needs AWS credentials and a queue):
//
//	QUEUE_URL=https://sqs.eu-west-1.amazonaws.com/123/events go run .
package main

import (
	"context"
	"log"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/duffleone/dfl/events"
	"github.com/duffleone/dfl/events/aws/sqs"
)

// UserCreated is the example event.
type UserCreated struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func main() {
	ctx := context.Background()

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	sink := sqs.NewSink(awssqs.NewFromConfig(cfg), os.Getenv("QUEUE_URL"))
	bus := events.NewBus(sink)

	bus.On(func(_ context.Context, e UserCreated) error {
		log.Printf("welcome %s", e.Email)

		return nil
	})

	// Emit sends to the queue; Receive (below) reads it back and handles it.
	if err := bus.Emit(ctx, UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		log.Fatalf("emit: %v", err)
	}

	log.Println("receiving from SQS...")
	log.Fatal(sink.Receive(ctx)) // blocks until ctx is cancelled
}
