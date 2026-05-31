// Command sns publishes events to Amazon SNS and receives them back over an SNS
// HTTP push subscription. The sink is an http.Handler, so it mounts on any mux.
//
// Run (needs AWS credentials, a topic, and a public URL SNS can reach, e.g. via
// a tunnel):
//
//	TOPIC_ARN=arn:aws:sns:eu-west-1:123:events go run .
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/duffleone/dfl/events"
	"github.com/duffleone/dfl/events/aws/sns"
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

	topics := map[string]string{"user.created": os.Getenv("TOPIC_ARN")}
	sink := sns.NewPushSink(awssns.NewFromConfig(cfg), topics)

	bus := events.NewBus(sink)
	bus.On(func(_ context.Context, e UserCreated) error {
		log.Printf("welcome %s", e.Email)

		return nil
	})

	// Publish one. It travels through SNS and comes back to the endpoint below.
	if err := bus.Emit(ctx, UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		log.Fatalf("emit: %v", err)
	}

	// Point your SNS HTTPS subscription at POST /events/sns. The sink confirms
	// the subscription handshake and dispatches notifications to the bus.
	mux := http.NewServeMux()
	mux.Handle("POST /events/sns", sink)

	log.Println("listening on :8080 for SNS deliveries")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
