// Package eventbridge provides an events publisher and an HTTP push ingress for
// Amazon EventBridge, the more complex AWS event bus. Publish puts an event on
// the bus with the event name as its detail-type; the push sink receives events
// routed to an HTTP target (an API destination) and dispatches them.
//
// EventBridge does not let you pull directly. Delivery is via rules that route
// to targets: an SQS queue (pull it with the sqs package) or an HTTP API
// destination (push it here).
package eventbridge

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/duffleone/dfl/events"
)

// PutAPI is the slice of *eventbridge.Client the publisher calls.
type PutAPI interface {
	PutEvents(ctx context.Context, in *eventbridge.PutEventsInput, optFns ...func(*eventbridge.Options)) (*eventbridge.PutEventsOutput, error)
}

// Publisher puts events on an EventBridge bus. The event name becomes the
// detail-type and the payload the detail; source is a fixed string identifying
// this producer.
type Publisher struct {
	client  PutAPI
	busName string
	source  string
}

// NewPublisher builds a Publisher for busName, tagging events with source.
func NewPublisher(client PutAPI, busName, source string) *Publisher {
	return &Publisher{client: client, busName: busName, source: source}
}

// Publish puts env on the bus and returns once EventBridge accepts it. A
// per-entry failure (EventBridge reports these without failing the call) is
// surfaced as an error.
func (p *Publisher) Publish(ctx context.Context, env events.Envelope) error {
	out, err := p.client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{{
			EventBusName: aws.String(p.busName),
			Source:       aws.String(p.source),
			DetailType:   aws.String(env.Name),
			Detail:       aws.String(string(env.Payload)),
		}},
	})
	if err != nil {
		return events.Wrap(err, "publish_failed", events.M{"event": env.Name})
	}

	if out.FailedEntryCount > 0 {
		meta := events.M{"event": env.Name}
		if len(out.Entries) > 0 && out.Entries[0].ErrorMessage != nil {
			meta["error"] = *out.Entries[0].ErrorMessage
		}

		return events.New("publish_failed", meta)
	}

	return nil
}

// PushSink is an events.Sink that publishes to EventBridge and receives events
// delivered to an HTTP API destination. Register it with the bus and mount it
// as an http.Handler.
type PushSink struct {
	*events.Dispatcher
	*Publisher
}

var (
	_ events.Sink  = (*PushSink)(nil)
	_ http.Handler = (*PushSink)(nil)
)

// NewPushSink builds a push sink that publishes to busName and accepts
// EventBridge events over HTTP.
func NewPushSink(client PutAPI, busName, source string) *PushSink {
	return &PushSink{
		Dispatcher: events.NewDispatcher(),
		Publisher:  NewPublisher(client, busName, source),
	}
}

// ebEvent is the EventBridge event envelope delivered to an HTTP target.
type ebEvent struct {
	DetailType string          `json:"detail-type"`
	Detail     json.RawMessage `json:"detail"`
}

// ServeHTTP dispatches an EventBridge event to the handlers registered for its
// detail-type. A 2xx acks; a 5xx makes EventBridge retry per the target's retry
// policy.
func (s *PushSink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var e ebEvent
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		http.Error(w, "invalid eventbridge event", http.StatusBadRequest)

		return
	}

	env := events.Envelope{Name: e.DetailType, Payload: e.Detail}

	if err := s.Dispatch(r.Context(), env); err != nil {
		http.Error(w, "handler failed", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}
