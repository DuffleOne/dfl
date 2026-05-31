// Example program: the smallest events round trip on the in-memory sink.
//
// Run:
//
//	go run ./events/examples/basic
package main

import (
	"context"
	"log"
	"sync"

	"github.com/duffleone/dfl/events"
)

// UserCreated is an event. It names itself; that name is the topic On and Emit
// use under the hood.
type UserCreated struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func main() {
	bus := events.NewBus(events.NewMemSink())

	var wg sync.WaitGroup
	wg.Add(1)

	bus.On(func(_ context.Context, e UserCreated) error {
		log.Printf("welcome %s (%s)", e.Email, e.ID)
		wg.Done()

		return nil
	})

	if err := bus.Emit(context.Background(), UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		log.Fatalf("emit: %v", err)
	}

	// On delivery is async, so wait for the handler before the program exits.
	wg.Wait()
}
