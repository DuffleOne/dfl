package events

import "context"

// Sink is the transport a Bus publishes through and receives from. It plays the
// role Mux plays in the http package: the bus has no awareness of any specific
// transport, it just registers deliver callbacks and hands envelopes over.
//
// MemSink, the in-memory implementation, ships in this package. External sinks
// (a GCP Pub/Sub topic, a NATS subject, a pgxdb outbox table, an HTTP webhook
// fan-out) implement the same two methods. See events/examples/pubsub for a
// reference Pub/Sub sink.
//
// Publish must not report success until the event is certain to be delivered:
// for MemSink that means every subscriber goroutine has been launched, and for
// a durable transport it means the broker has acknowledged the publish. Emit
// relies on that guarantee, it blocks on Publish and returns once the event is
// committed for delivery.
//
// Subscribe registers a deliver callback for an event name. It's a boot-time
// call, like registering an http route; a sink isn't expected to handle a high
// churn of subscriptions once traffic is flowing. The callback returns nil when
// the event was handled and a non-nil error when delivery failed; the bus has
// already reported that error to its ErrorHandler, but a durable sink can use
// the return to nack and have the message redelivered. MemSink ignores it.
type Sink interface {
	Publish(ctx context.Context, env Envelope) error
	Subscribe(name string, deliver HandlerFunc)
}
