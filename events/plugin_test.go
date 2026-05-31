package events_test

import (
	"context"
	"sync"
	"testing"

	"github.com/duffleone/dfl/events"
)

// TestPluginPropagatesHeader installs a plugin that stamps a header on publish
// and reads it back on deliver, proving the produce/consume hooks and the
// Envelope.Headers carrier work end to end through MemSink. This is the shape
// OpenTelemetry uses: inject trace context on publish, extract it on deliver.
func TestPluginPropagatesHeader(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	var seen string

	plugin := events.PluginFuncs{
		Publish: func(next events.PublishFunc) events.PublishFunc {
			return func(ctx context.Context, env *events.Envelope) error {
				env.Headers["trace"] = "abc123"

				return next(ctx, env)
			}
		},
		Deliver: func(next events.HandlerFunc) events.HandlerFunc {
			return func(ctx context.Context, env events.Envelope) error {
				seen = env.Headers["trace"]
				wg.Done()

				return next(ctx, env)
			}
		},
	}

	bus := events.NewBus(events.NewMemSink(), events.WithPlugins(plugin))
	bus.On(func(context.Context, evtPing) error { return nil })

	if err := bus.Emit(context.Background(), evtPing{Seq: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	wg.Wait()

	if seen != "abc123" {
		t.Errorf("deliver saw trace header %q, want abc123", seen)
	}
}

// TestPluginFuncsPublishOnly checks a plugin can opt out of the consume side: a
// nil Deliver leaves delivery unchanged while the publish side still runs.
func TestPluginFuncsPublishOnly(t *testing.T) {
	called := false

	plugin := events.PluginFuncs{
		Publish: func(next events.PublishFunc) events.PublishFunc {
			return func(ctx context.Context, env *events.Envelope) error {
				called = true

				return next(ctx, env)
			}
		},
	}

	bus := events.NewBus(events.NewMemSink(), events.WithPlugins(plugin))

	if err := bus.Emit(context.Background(), evtPing{Seq: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if !called {
		t.Error("publish middleware was not called")
	}
}
