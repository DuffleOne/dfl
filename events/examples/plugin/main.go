// Command plugin shows how to write and install a Bus plugin. The plugin here
// assigns each event a correlation id on the way out, carries it in the
// envelope headers, and reads it back on the way in, putting it on the context
// the handler sees. That's the whole plugin system in one place: the produce
// hook, the consume hook, and the Envelope.Headers carrier that links them.
// OpenTelemetry works exactly the same way; see events/otel.
//
// Run:
//
//	go run ./events/examples/plugin
package main

import (
	"context"
	"log"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/duffleone/dfl/events"
)

const correlationHeader = "correlation-id"

// correlationPlugin stamps a correlation id onto each published event and
// surfaces it on delivery. It implements events.Plugin directly; for a one-sided
// plugin, events.PluginFuncs{Deliver: ...} is a shorter route.
type correlationPlugin struct {
	seq atomic.Int64
}

var _ events.Plugin = (*correlationPlugin)(nil)

// WrapPublish stamps a fresh correlation id into the outgoing envelope's
// headers, where it travels with the event.
func (p *correlationPlugin) WrapPublish(next events.PublishFunc) events.PublishFunc {
	return func(ctx context.Context, env *events.Envelope) error {
		id := "req-" + strconv.FormatInt(p.seq.Add(1), 10)
		env.Headers[correlationHeader] = id

		log.Printf("publish %s [%s]", env.Name, id)

		return next(ctx, env)
	}
}

// WrapDeliver reads the correlation id the producer set and hands it to the
// handler on the context.
func (p *correlationPlugin) WrapDeliver(next events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, env events.Envelope) error {
		id := env.Headers[correlationHeader]

		log.Printf("deliver %s [%s]", env.Name, id)

		return next(withCorrelationID(ctx, id), env)
	}
}

type correlationKey struct{}

func withCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey{}, id)
}

func correlationID(ctx context.Context) string {
	id, _ := ctx.Value(correlationKey{}).(string)

	return id
}

// UserCreated is the example event.
type UserCreated struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func main() {
	bus := events.NewBus(events.NewMemSink(), events.WithPlugins(&correlationPlugin{}))

	var wg sync.WaitGroup
	wg.Add(2)

	bus.On(func(ctx context.Context, e UserCreated) error {
		log.Printf("welcome %s (correlation %s)", e.Email, correlationID(ctx))
		wg.Done()

		return nil
	})

	// Two events get two correlation ids, each carried from publish to handler.
	for _, u := range []UserCreated{{ID: "1", Email: "a@b.com"}, {ID: "2", Email: "b@c.com"}} {
		if err := bus.Emit(context.Background(), u); err != nil {
			log.Fatalf("emit: %v", err)
		}
	}

	wg.Wait()
}
