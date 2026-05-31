package events_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/duffleone/dfl/events"
	"github.com/duffleone/dfl/events/internal/bustest"
)

// TestMemSinkConformance runs the shared Sink conformance suite against the
// in-memory backend. routertest in the http package has no caller; this one
// does, so the Sink contract is exercised, not just compiled.
func TestMemSinkConformance(t *testing.T) {
	bustest.Run(t, bustest.Factory{
		New: func(opts ...events.Option) (*events.Bus, events.Sink) {
			s := events.NewMemSink()

			return events.NewBus(s, opts...), s
		},
	})
}

// TestMemSinkDetachedContext checks the mem-specific guarantee that a handler
// keeps running on a context that is not cancelled when the emitter's context
// is. The handler blocks until the parent ctx has been cancelled, then reports
// its own ctx error, which must be nil.
func TestMemSinkDetachedContext(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())

	proceed := make(chan struct{})
	result := make(chan error, 1)

	bus.On(func(ctx context.Context, _ evtPing) error {
		<-proceed
		result <- ctx.Err()

		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := bus.Emit(ctx, evtPing{Seq: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	cancel()       // cancel the parent after Emit has returned
	close(proceed) // let the handler observe its own context

	select {
	case err := <-result:
		if err != nil {
			t.Errorf("handler ctx err = %v, want nil (context should be detached)", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not run")
	}
}

// TestMemSinkConcurrentSubscribeAndPublish exercises the mutex on the subscriber
// map. Meaningful under -race.
func TestMemSinkConcurrentSubscribeAndPublish(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())

	var wg sync.WaitGroup

	for range 20 {
		wg.Add(2)

		go func() {
			defer wg.Done()

			bus.On(func(_ context.Context, _ evtPing) error { return nil })
		}()

		go func() {
			defer wg.Done()

			_ = bus.Emit(context.Background(), evtPing{Seq: 1})
		}()
	}

	wg.Wait()
}
