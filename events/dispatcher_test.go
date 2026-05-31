package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/duffleone/dfl/events"
)

func TestDispatcherRunsAllHandlers(t *testing.T) {
	d := events.NewDispatcher()

	var a, b bool
	d.Subscribe("test.ping", func(_ context.Context, _ events.Envelope) error { a = true; return nil })
	d.Subscribe("test.ping", func(_ context.Context, _ events.Envelope) error { b = true; return nil })

	err := d.Dispatch(context.Background(), events.Envelope{Name: "test.ping"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if !a || !b {
		t.Errorf("not all handlers ran: a=%v b=%v", a, b)
	}
}

func TestDispatcherJoinsErrors(t *testing.T) {
	d := events.NewDispatcher()

	boom := errors.New("boom")
	d.Subscribe("test.ping", func(_ context.Context, _ events.Envelope) error { return nil })
	d.Subscribe("test.ping", func(_ context.Context, _ events.Envelope) error { return boom })

	err := d.Dispatch(context.Background(), events.Envelope{Name: "test.ping"})
	if !errors.Is(err, boom) {
		t.Errorf("dispatch err = %v, want it to wrap boom", err)
	}
}

func TestDispatcherUnknownEventIsNoop(t *testing.T) {
	d := events.NewDispatcher()

	if err := d.Dispatch(context.Background(), events.Envelope{Name: "nope"}); err != nil {
		t.Errorf("dispatch of unknown event = %v, want nil", err)
	}
}
