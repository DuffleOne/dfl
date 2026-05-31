package events_test

import (
	"context"
	"sync"
	"testing"

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

// TestMemSinkDetachedContext checks the mem-specific guarantee that delivery
// runs on a context that is not cancelled with the emitter's. The parent
// context is already cancelled at Emit time, yet the handler's context must not
// be.
func TestMemSinkDetachedContext(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())

	var wg sync.WaitGroup
	wg.Add(1)

	var handlerErr error
	bus.On(func(ctx context.Context, _ evtPing) error {
		handlerErr = ctx.Err()
		wg.Done()

		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel the parent before publishing

	if err := bus.Emit(ctx, evtPing{Seq: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	wg.Wait()

	if handlerErr != nil {
		t.Errorf("handler ctx err = %v, want nil (context should be detached)", handlerErr)
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
