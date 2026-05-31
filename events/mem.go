package events

import (
	"context"
	"sync"
)

// MemSink is the in-process Sink, the default backend, analogous to reaching
// for *http.ServeMux with the http package's Router. Publish fans an event out
// to every subscriber registered for its name, each on its own goroutine.
//
// Delivery is asynchronous: Publish launches the goroutines and returns, so by
// the time it returns the event is certain to fire, but the handlers themselves
// run in the background and in no guaranteed order. Errors a deliver returns are
// dropped here; the bus's deliver closure routes handler errors to the bus
// ErrorHandler before they ever reach the sink.
type MemSink struct {
	mu   sync.RWMutex
	subs map[string][]HandlerFunc
}

var _ Sink = (*MemSink)(nil)

// NewMemSink returns an empty in-memory sink.
func NewMemSink() *MemSink {
	return &MemSink{subs: map[string][]HandlerFunc{}}
}

// Subscribe registers deliver for the named event. Expected at boot, before
// Publish traffic; guarded so a late registration can't race a publish.
func (s *MemSink) Subscribe(name string, deliver HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subs[name] = append(s.subs[name], deliver)
}

// Publish delivers env to every subscriber of env.Name, each on its own
// goroutine, then returns. The goroutines run on a context derived with
// context.WithoutCancel so a handler isn't cancelled when the emitter's ctx
// ends. An event with no subscribers is a no-op.
func (s *MemSink) Publish(ctx context.Context, env Envelope) error {
	s.mu.RLock()
	subs := append([]HandlerFunc(nil), s.subs[env.Name]...)
	s.mu.RUnlock()

	deliverCtx := context.WithoutCancel(ctx)

	for _, deliver := range subs {
		go func() { _ = deliver(deliverCtx, env) }()
	}

	return nil
}
