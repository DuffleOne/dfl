//go:build ignore

// Command pubsub is a reference events.Sink backed by Google Cloud Pub/Sub.
//
// It is deliberately excluded from the module build by the //go:build ignore
// constraint above, so github.com/duffleone/dfl never depends on
// cloud.google.com/go/pubsub. The events library stays small; you pull Pub/Sub
// in only in the service that needs it. To run this, copy it into a project
// that has the dependency:
//
//	go get cloud.google.com/go/pubsub
//	go run .
//
// What it shows is how the Sink contract maps onto Pub/Sub:
//
//   - Publish sends the envelope to the event's topic and blocks on the server
//     ack (result.Get). That is what makes Emit's "certain to fire" guarantee
//     hold across the network: Emit returns only once Pub/Sub has the message
//     durably.
//   - Subscribe starts a streaming pull and hands each message to deliver. On
//     success it acks; on error it nacks, so Pub/Sub redelivers. The bus has
//     already reported the error to its ErrorHandler.
//
// The Envelope crosses the wire unchanged: Emit encodes the event into
// env.Payload with the bus codec, and the consuming process decodes it the same
// way. Producer and consumer run identical code to the in-memory case; only the
// transport differs.
package main

import (
	"context"
	"log"

	"cloud.google.com/go/pubsub"

	"github.com/duffleone/dfl/events"
)

// PubSubSink is an events.Sink over a Pub/Sub project, with one topic and one
// subscription per event name. Topics and subscriptions are assumed to exist
// already (create them with Terraform, gcloud, or the admin API).
type PubSubSink struct {
	ctx    context.Context // base context bounding the receive loops
	client *pubsub.Client
	// consumer names this service's subscriptions: event "user.created"
	// consumed by "welcome-service" pulls from subscription
	// "user.created.welcome-service". Keep it stable per deployment.
	consumer string
}

var _ events.Sink = (*PubSubSink)(nil)

// NewPubSubSink builds a sink. ctx bounds the lifetime of the receive loops
// started by Subscribe; cancel it to shut delivery down.
func NewPubSubSink(ctx context.Context, client *pubsub.Client, consumer string) *PubSubSink {
	return &PubSubSink{ctx: ctx, client: client, consumer: consumer}
}

// Publish sends env to the event's topic and blocks until Pub/Sub acks it, so
// the caller of Emit knows the event is committed for delivery.
func (s *PubSubSink) Publish(ctx context.Context, env events.Envelope) error {
	topic := s.client.Topic(topicID(env.Name))

	result := topic.Publish(ctx, &pubsub.Message{
		Data:       env.Payload,
		Attributes: map[string]string{"event": env.Name},
	})

	// Get blocks until Pub/Sub confirms the publish, or fails it.
	if _, err := result.Get(ctx); err != nil {
		return events.Wrap(err, "publish_failed", events.M{"event": env.Name})
	}

	return nil
}

// Subscribe starts a streaming pull for the event's subscription and forwards
// each message to deliver, acking on success and nacking on failure. It runs on
// its own goroutine until the sink's context is cancelled.
func (s *PubSubSink) Subscribe(name string, deliver events.HandlerFunc) {
	sub := s.client.Subscription(s.subscriptionID(name))

	go func() {
		err := sub.Receive(s.ctx, func(ctx context.Context, m *pubsub.Message) {
			if derr := deliver(ctx, events.Envelope{Name: name, Payload: m.Data}); derr != nil {
				m.Nack() // let Pub/Sub redeliver

				return
			}

			m.Ack()
		})
		if err != nil && s.ctx.Err() == nil {
			log.Printf("pubsub: receive on %q stopped: %v", name, err)
		}
	}()
}

// topicID maps an event name to a Pub/Sub topic id. Real topic ids have a
// limited charset, so sanitise here if your event names need it (the same idea
// as the http endpoint's pathSafe).
func topicID(eventName string) string { return eventName }

func (s *PubSubSink) subscriptionID(eventName string) string {
	return eventName + "." + s.consumer
}

// UserCreated is the example event. Its json tags are its wire form, the same
// bytes that travel in the Pub/Sub message.
type UserCreated struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func main() {
	ctx := context.Background()

	client, err := pubsub.NewClient(ctx, "my-gcp-project")
	if err != nil {
		log.Fatalf("pubsub client: %v", err)
	}
	defer func() { _ = client.Close() }()

	bus := events.NewBus(NewPubSubSink(ctx, client, "welcome-service"))

	// Consumer side: handle user.created as it arrives from Pub/Sub. Same
	// bus.On call you'd make with the in-memory sink.
	bus.On(func(_ context.Context, e UserCreated) error {
		log.Printf("welcome %s", e.Email)

		return nil
	})

	// Producer side: publish one. Emit blocks until Pub/Sub acks the publish.
	if err := bus.Emit(ctx, UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		log.Fatalf("emit: %v", err)
	}

	select {} // block so the subscriber keeps receiving; cancel ctx to stop
}
