package events_test

import (
	"context"
	"errors"
	"testing"

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

func (evtOrder) EventName() string  { return "order.shipped" }
func (evtOrder) URLSafeName() string { return "orders-shipped" }

func TestEmitStampsEventNameOnError(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())

	err := bus.Emit(context.Background(), evtUser{Email: ""})

	var eventErr *events.EventError
	if !errors.As(err, &eventErr) {
		t.Fatalf("emit error = %v, want *EventError", err)
	}

	if eventErr.Event != "test.user" {
		t.Errorf("stamped event = %q, want test.user", eventErr.Event)
	}
}

func TestEmitNoSubscribersReturnsNil(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())

	if err := bus.Emit(context.Background(), evtPing{Seq: 1}); err != nil {
		t.Errorf("emit = %v, want nil", err)
	}
}
