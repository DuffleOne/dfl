// Command trace shows OpenTelemetry trace propagation through the events bus,
// using a stdout exporter so you can see the producer span ("user.created
// publish") and the consumer span ("user.created process") share a trace id.
//
// Run:
//
//	go run .
package main

import (
	"context"
	"log"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/duffleone/dfl/events"
	otelevents "github.com/duffleone/dfl/events/otel"
)

// UserCreated is the example event.
type UserCreated struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func main() {
	// Wire up a TracerProvider that prints spans, and the W3C propagator. This
	// is normal app setup; the events plugin just uses whatever you register.
	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatalf("exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// One line wires tracing into the bus.
	bus := events.NewBus(events.NewMemSink(), events.WithPlugins(otelevents.New()))

	var wg sync.WaitGroup
	wg.Add(1)

	bus.On(func(_ context.Context, e UserCreated) error {
		log.Printf("welcome %s", e.Email)
		wg.Done()

		return nil
	})

	if err := bus.Emit(context.Background(), UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		log.Fatalf("emit: %v", err)
	}

	wg.Wait() // let the consumer span finish before Shutdown flushes
}
