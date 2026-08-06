package events

import (
	"context"
	"errors"
	"sync"
)

// Dispatcher is a synchronous Subscribe/Dispatch registry for sinks that
// receive events out of band and want one error back to decide an ack: a
// pull sink embeds it for Subscribe and calls Dispatch per message, and a
// push sink does the same from its HTTP handler. Dispatch runs every
// handler registered for the name and joins their errors, so a fan-out
// nacks if any handler fails (redelivery may re-run the ones that passed).
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
