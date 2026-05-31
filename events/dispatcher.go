package events

import (
	"context"
	"errors"
	"sync"
)

// Dispatcher is a synchronous Subscribe/Dispatch registry for sinks that
// receive events out of band, where the receiving side has a raw (name,
// payload) in hand and wants a single error back to decide whether to ack.
//
// It's the building block the cloud adapters use. A pull sink (an SQS queue, a
// Pub/Sub streaming pull) embeds a Dispatcher to satisfy Subscribe, and its
// receive loop calls Dispatch for each message, deleting or nacking on the
// result. A push sink (an SNS or Pub/Sub HTTP push) does the same from its HTTP
// handler. Delivery is synchronous, unlike MemSink, because the caller needs
// the outcome to ack or redeliver.
//
// Dispatch runs every handler registered for the event name and joins their
// errors, so a single message that fans out to several handlers nacks if any of
// them fail (at-least-once redelivery may then re-run the ones that succeeded).
type Dispatcher struct {
	mu   sync.RWMutex
	subs map[string][]HandlerFunc
}

// NewDispatcher returns an empty Dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{subs: map[string][]HandlerFunc{}}
}

// Subscribe registers deliver for the named event. It satisfies the Subscribe
// half of the Sink interface.
func (d *Dispatcher) Subscribe(name string, deliver HandlerFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.subs[name] = append(d.subs[name], deliver)
}

// Dispatch runs every handler registered for env.Name synchronously and returns
// their joined error (nil when there are none or all succeed). An event with no
// handlers is a no-op returning nil.
func (d *Dispatcher) Dispatch(ctx context.Context, env Envelope) error {
	d.mu.RLock()
	subs := append([]HandlerFunc(nil), d.subs[env.Name]...)
	d.mu.RUnlock()

	var errs []error

	for _, deliver := range subs {
		if err := deliver(ctx, env); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
