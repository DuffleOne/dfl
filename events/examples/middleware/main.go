// Example: bus middleware (logging + recover), a custom error handler, and a
// pluggable validator swapped in with WithValidator.
//
// Run:
//
//	go run ./events/examples/middleware
package main

import (
	"context"
	"log"
	"sync"

	"github.com/duffleone/dfl/events"
)

// Beat is a heartbeat event.
type Beat struct {
	N int `json:"n"`
}

func (Beat) EventName() string { return "demo.beat" }

// logging logs each event as it flows through, and its outcome.
func logging(next events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, env events.Envelope) error {
		log.Printf("-> %s", env.Name)

		err := next(ctx, env)
		if err != nil {
			log.Printf("<- %s: %v", env.Name, err)
		}

		return err
	}
}

// recoverMW turns a handler panic into an error so it reaches the ErrorHandler
// instead of crashing the delivery goroutine.
func recoverMW(next events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, env events.Envelope) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = events.New("panic", events.M{"recovered": r})
			}
		}()

		return next(ctx, env)
	}
}

// rejectOdd is a custom Validator: it only allows even beats. Swapped in via
// WithValidator, replacing the default Validate()-calling validator.
type rejectOdd struct{}

func (rejectOdd) Validate(e events.Event) error {
	if b, ok := e.(Beat); ok && b.N%2 != 0 {
		return events.New("odd_beat", events.M{"n": b.N})
	}

	return nil
}

func main() {
	// wg tracks the two async deliveries that finish: one success, one that
	// panics and lands in the error handler. The program waits for both before
	// exiting.
	var wg sync.WaitGroup
	wg.Add(2)

	bus := events.NewBus(
		events.NewMemSink(),
		events.WithValidator(rejectOdd{}),
		events.WithErrorHandler(func(_ context.Context, env events.Envelope, err *events.EventError) {
			log.Printf("error handler: %s failed with %s", env.Name, err.Code)
			wg.Done()
		}),
	)

	bus.Use(logging, recoverMW)

	bus.On(func(_ context.Context, b Beat) error {
		if b.N == 4 {
			panic("boom")
		}

		log.Printf("beat %d", b.N)
		wg.Done()

		return nil
	})

	// Even, handled cleanly.
	_ = bus.Emit(context.Background(), Beat{N: 2})

	// Even, but the handler panics: recoverMW turns it into an error that
	// reaches the ErrorHandler.
	_ = bus.Emit(context.Background(), Beat{N: 4})

	// Odd, rejected by the custom validator at Emit, so it's never published.
	if err := bus.Emit(context.Background(), Beat{N: 3}); err != nil {
		log.Printf("emit rejected before publish: %v", err)
	}

	wg.Wait()
}
