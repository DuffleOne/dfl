package events_test

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"strings"
	"testing"

	"github.com/duffleone/dfl/events"
)

// syncSink delivers on the publishing goroutine, so a test can read what a
// delivery logged as soon as Emit returns.
type syncSink struct{ *events.Dispatcher }

func (s syncSink) Publish(ctx context.Context, env events.Envelope) error {
	_ = s.Dispatch(ctx, env)

	return nil
}

// failingBus is a bus over a syncSink whose one handler never finds its
// row, retried once: a delivery that logs from Retry and the ErrorHandler.
func failingBus(opts ...events.Option) *events.Bus {
	bus := events.NewBus(syncSink{events.NewDispatcher()}, opts...)
	bus.Use(events.Retry(events.RetryPolicy{Attempts: 1}))

	bus.On(func(context.Context, testUserCreated) error {
		return events.NotFound
	})

	return bus
}

// wantBusLogs checks got for both lines a failed delivery writes.
func wantBusLogs(t *testing.T, got string) {
	t.Helper()

	for _, want := range []string{
		`msg="events: retries exhausted"`, `msg="events: handler failed"`, "event=test.user.created",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log = %q, want it to contain %q", got, want)
		}
	}
}

// TestWithLoggerHearsTheBus: the default ErrorHandler and Retry running out
// of attempts both log to the bus's logger, Retry finding it on the
// delivery's context.
func TestWithLoggerHearsTheBus(t *testing.T) {
	var buf bytes.Buffer

	bus := failingBus(events.WithLogger(slog.New(slog.NewTextHandler(&buf, nil))))

	if err := bus.Emit(t.Context(), testUserCreated{ID: "1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	wantBusLogs(t, buf.String())
}

// TestBusLogsThroughTheDefaultOfTheMoment: with no WithLogger the bus reads
// slog.Default() as it logs, so a default set after NewBus still hears it.
func TestBusLogsThroughTheDefaultOfTheMoment(t *testing.T) {
	bus := failingBus()

	// SetDefault rewires the log package too, so all three go back after.
	prev, out, flags := slog.Default(), log.Writer(), log.Flags()

	t.Cleanup(func() {
		slog.SetDefault(prev)
		log.SetOutput(out)
		log.SetFlags(flags)
	})

	var buf bytes.Buffer

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	if err := bus.Emit(t.Context(), testUserCreated{ID: "1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	wantBusLogs(t, buf.String())
}
