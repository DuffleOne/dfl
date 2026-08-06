package events

import (
	"context"
	"sync"
)

// MemSink is the in-process Sink, the default backend, as *http.ServeMux
// is to the Router. Publish fans out to every subscriber of the event's
// name, each on its own goroutine, and returns once they're launched: the
// event is then certain to fire, though handlers run in the background in
// no guaranteed order. Deliver errors are dropped here, the bus having
// already routed them to the ErrorHandler; a durable transport nacks.
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
