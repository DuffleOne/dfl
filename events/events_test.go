package events_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/duffleone/dfl/events"
)

// Shared event types for the black-box tests in package events_test.

// evtPing has no Validate method, so the default validator is a no-op for it.
type evtPing struct {
	Seq int `json:"seq"`
}

func (evtPing) EventName() string { return "test.ping" }

// evtUser validates itself: Email is required.
type evtUser struct {
	Email string `json:"email"`
}

func (evtUser) EventName() string { return "test.user" }

func (e evtUser) Validate() error {
	if e.Email == "" {
		return events.New("validation_failed", events.M{"fields": events.M{"email": "is required"}})
	}

	return nil
}

// evtOrder pins a custom HTTP route segment via URLSafeName.
type evtOrder struct {
	ID string `json:"id"`
}

func (evtOrder) EventName() string   { return "order.shipped" }
func (evtOrder) URLSafeName() string { return "orders-shipped" }

func TestEmitStampsEventNameOnError(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())

	err := bus.Emit(t.Context(), evtUser{Email: ""})

	var eventErr *events.EventError
	if !errors.As(err, &eventErr) {
		t.Fatalf("emit error = %v, want *EventError", err)
	}

	if eventErr.Event != "test.user" {
		t.Errorf("stamped event = %q, want test.user", eventErr.Event)
	}
}

// TestStampingLeavesTheSentinelAlone: handlers return package-level sentinels
// directly (events.NotFound, or one of their own), so stamping the event name
// has to copy. Mutating would race between concurrent deliveries and leave one
// event's name on the error a different event later returns.
func TestStampingLeavesTheSentinelAlone(t *testing.T) {
	sentinel := events.New("too_early", nil)
	seen := make(chan *events.EventError, 1)

	bus := events.NewBus(events.NewMemSink(),
		events.WithErrorHandler(func(_ context.Context, _ events.Envelope, err *events.EventError) {
			seen <- err
		}))

	bus.On(func(context.Context, evtUser) error { return sentinel })

	if err := bus.Emit(t.Context(), evtUser{Email: "a@b.com"}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	select {
	case got := <-seen:
		if got.Event != "test.user" {
			t.Errorf("handled error event = %q, want test.user", got.Event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("error handler never ran")
	}

	if sentinel.Event != "" {
		t.Errorf("sentinel was stamped with %q; it must not be mutated", sentinel.Event)
	}
}

func TestEmitNoSubscribersReturnsNil(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())

	if err := bus.Emit(t.Context(), evtPing{Seq: 1}); err != nil {
		t.Errorf("emit = %v, want nil", err)
	}
}
