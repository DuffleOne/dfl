// Package pubsub provides events transports backed by Google Cloud Pub/Sub, in
// both flavours so a deployment can pick:
//
//   - PullSink streams messages with a background receiver, acking on success
//     and nacking on failure. Run it as a worker.
//   - PushSink is an http.Handler for a Pub/Sub push subscription, so Pub/Sub
//     POSTs deliveries straight to your server. Run it behind an endpoint.
//
// Both publish to a topic per event name, tagging the name in the message's
// "event" attribute. Publish blocks on the server ack, which is the delivery
// guarantee Emit relies on.
package pubsub

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"sync"

	"cloud.google.com/go/pubsub"

	"github.com/duffleone/dfl/events"
)

const (
	eventAttr   = "event"
	headersAttr = "headers"
)

// publish sends env to its topic and blocks until Pub/Sub acks it.
func publish(ctx context.Context, client *pubsub.Client, env events.Envelope) error {
	attrs := map[string]string{eventAttr: env.Name}
	if h := marshalHeaders(env.Headers); h != "" {
		attrs[headersAttr] = h
	}

	result := client.Topic(env.Name).Publish(ctx, &pubsub.Message{
		Data:       env.Payload,
		Attributes: attrs,
	})

	if _, err := result.Get(ctx); err != nil {
		return events.Wrap(err, "publish_failed", events.M{"event": env.Name})
	}

	return nil
}

// PullSink is an events.Sink that publishes to Pub/Sub and receives via a
// background streaming pull, one receiver per subscribed event. Subscribe
// records the handlers; Receive starts the pulls and takes the context, so the
// sink doesn't hold one.
type PullSink struct {
	client   *pubsub.Client
	consumer string

	mu   sync.Mutex
	subs map[string]events.HandlerFunc
}

var _ events.Sink = (*PullSink)(nil)

// NewPullSink builds a pull sink. consumer names this service's subscriptions,
// so event "user.created" pulls from subscription "user.created.<consumer>".
func NewPullSink(client *pubsub.Client, consumer string) *PullSink {
	return &PullSink{
		client:   client,
		consumer: consumer,
		subs:     map[string]events.HandlerFunc{},
	}
}

// Publish sends env to the event's topic and blocks on the ack.
func (s *PullSink) Publish(ctx context.Context, env events.Envelope) error {
	return publish(ctx, s.client, env)
}

// Subscribe records a handler for the event. Receive starts the actual pull.
func (s *PullSink) Subscribe(name string, deliver events.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subs[name] = deliver
}

// Receive starts a streaming pull for every subscribed event, each forwarding
// messages to its handler and acking or nacking on the result. It blocks until
// ctx is cancelled. Call it once, after the bus has registered its handlers.
func (s *PullSink) Receive(ctx context.Context) error {
	s.mu.Lock()
	subs := make(map[string]events.HandlerFunc, len(s.subs))
	maps.Copy(subs, s.subs)
	s.mu.Unlock()

	var wg sync.WaitGroup

	for name, deliver := range subs {
		wg.Add(1)

		go func() {
			defer wg.Done()

			s.receive(ctx, name, deliver)
		}()
	}

	wg.Wait()

	return ctx.Err()
}

func (s *PullSink) receive(ctx context.Context, name string, deliver events.HandlerFunc) {
	sub := s.client.Subscription(name + "." + s.consumer)

	err := sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		env := events.Envelope{
			Name:    name,
			Payload: m.Data,
			Headers: unmarshalHeaders(m.Attributes[headersAttr]),
		}

		if derr := deliver(ctx, env); derr != nil {
			m.Nack()

			return
		}

		m.Ack()
	})
	if err != nil && ctx.Err() == nil {
		slog.ErrorContext(ctx, "pubsub: receive stopped",
			slog.String("event", name), slog.String("error", err.Error()))
	}
}

// PushSink is an events.Sink that publishes to Pub/Sub and receives via a push
// subscription, as an http.Handler. Register it with the bus and mount it on
// your server.
type PushSink struct {
	*events.Dispatcher

	client *pubsub.Client
}

var (
	_ events.Sink  = (*PushSink)(nil)
	_ http.Handler = (*PushSink)(nil)
)

// NewPushSink builds a push sink that publishes to Pub/Sub and accepts push
// deliveries over HTTP.
func NewPushSink(client *pubsub.Client) *PushSink {
	return &PushSink{Dispatcher: events.NewDispatcher(), client: client}
}

// Publish sends env to the event's topic and blocks on the ack.
func (s *PushSink) Publish(ctx context.Context, env events.Envelope) error {
	return publish(ctx, s.client, env)
}

// pushPayload is the JSON Pub/Sub POSTs to a push endpoint. Data is base64 on
// the wire; encoding/json decodes it into the byte slice automatically.
type pushPayload struct {
	Message struct {
		Data       []byte            `json:"data"`
		Attributes map[string]string `json:"attributes"`
	} `json:"message"`
}

// ServeHTTP dispatches a Pub/Sub push delivery to the handlers registered for
// the message's event attribute. A 2xx acks the message; a non-2xx makes
// Pub/Sub redeliver.
//
// Note: this does not verify the push request's OIDC token. Verify it in
// production so only Pub/Sub can deliver here.
func (s *PushSink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var p pushPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid push payload", http.StatusBadRequest)

		return
	}

	env := events.Envelope{
		Name:    p.Message.Attributes[eventAttr],
		Payload: p.Message.Data,
		Headers: unmarshalHeaders(p.Message.Attributes[headersAttr]),
	}

	if err := s.Dispatch(r.Context(), env); err != nil {
		http.Error(w, "handler failed", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// marshalHeaders encodes envelope headers to a JSON string for the "headers"
// message attribute, or "" when there are none.
func marshalHeaders(h map[string]string) string {
	if len(h) == 0 {
		return ""
	}

	b, err := json.Marshal(h)
	if err != nil {
		return ""
	}

	return string(b)
}

// unmarshalHeaders decodes the JSON string produced by marshalHeaders.
func unmarshalHeaders(s string) map[string]string {
	if s == "" {
		return nil
	}

	var h map[string]string
	if err := json.Unmarshal([]byte(s), &h); err != nil {
		return nil
	}

	return h
}
