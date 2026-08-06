package events

import "context"

// Sink is the transport a Bus publishes through and receives from, the
// role Mux plays in http: MemSink ships here, events/aws and events/gcp
// hold the cloud implementations, Dispatcher is the reusable Subscribe
// half. Publish must not report success until delivery is certain (broker
// ack, or subscribers scheduled): Emit's nil-means-it-will-fire rests on
// that. Subscribe is boot-time, and deliver's error is the nack signal.
type Sink interface {
	Publish(ctx context.Context, env Envelope) error
	Subscribe(name string, deliver HandlerFunc)
}
