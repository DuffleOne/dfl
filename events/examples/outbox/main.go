// Example program: the transactional outbox. Emit lands on the caller's
// transaction and a relay publishes after commit: no phantoms, none lost.
//
// Run:
//
//	DATABASE_URL=postgres://localhost/example go run ./events/examples/outbox
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/duffleone/dfl/db/pgxdb"
	"github.com/duffleone/dfl/events"
)

// UserCreated is the event. Note the ID travels with it: with the outbox the
// row is written first and the id is known by the time Emit runs, so there's
// no need to mint one up front the way emit-before-write requires.
type UserCreated struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func main() {
	ctx := context.Background()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("set DATABASE_URL")
	}

	db, err := pgxdb.New(ctx, url)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}

	defer db.Close()

	if err := setup(ctx, db); err != nil {
		log.Fatalf("setup: %v", err)
	}

	// The bus publishes through the outbox and consumes from the sink behind
	// it. Swap MemSink for an SQS or Pub/Sub sink and nothing above changes.
	outbox := newOutboxSink(db, events.NewMemSink())
	bus := events.NewBus(outbox)

	var wg sync.WaitGroup

	bus.On(func(_ context.Context, e UserCreated) error {
		defer wg.Done()

		log.Printf("consumer: user %d created (%s)", e.ID, e.Email)

		return nil
	})

	relayCtx, stopRelay := context.WithCancel(ctx)
	defer stopRelay()

	go func() {
		if err := outbox.Relay(relayCtx, 100*time.Millisecond, 50); err != nil &&
			!errors.Is(err, context.Canceled) {
			log.Printf("relay stopped: %v", err)
		}
	}()

	// The committed write: one transaction holds the user and the event.
	wg.Add(1)

	if err := createUser(ctx, db, bus, "ada@example.com", nil); err != nil {
		log.Fatalf("create: %v", err)
	}

	wg.Wait()

	// The rolled back write: the same code path, but the transaction fails
	// after the event was "published". The event row rolls back with the
	// user, so the relay never sees it and the consumer never hears about a
	// user that doesn't exist.
	wantErr := errors.New("charge declined")

	err = createUser(ctx, db, bus, "grace@example.com", func(context.Context) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		log.Fatalf("expected the rollback, got %v", err)
	}

	log.Printf("rolled back: no user, and no event either")

	// Give the relay a couple of ticks to prove nothing shows up.
	time.Sleep(300 * time.Millisecond)

	waiting, err := pgxdb.Scalar[int](ctx, db,
		`SELECT count(*) FROM event_outbox WHERE published_at IS NULL`)
	if err != nil {
		log.Fatalf("count: %v", err)
	}

	log.Printf("outbox backlog: %d", waiting)
}

// createUser writes the user and emits the event in one transaction. after
// runs inside the same transaction, standing in for whatever else the request
// does; returning an error from it rolls back both the user and the event.
func createUser(
	ctx context.Context,
	db *pgxdb.DB,
	bus *events.Bus,
	email string,
	after func(context.Context) error,
) error {
	return db.TxCtx(ctx, func(ctx context.Context) error {
		id, err := pgxdb.Scalar[int64](ctx, pgxdb.GetQuerier(ctx, db),
			`INSERT INTO users (email) VALUES ($1) RETURNING id`, email)
		if err != nil {
			return err
		}

		// Inside the transaction on purpose: Emit lands in the outbox table
		// on this same transaction. This is the line that would be a bug
		// against a direct transport sink and is correct against this one.
		if err := bus.Emit(ctx, UserCreated{ID: id, Email: email}); err != nil {
			return err
		}

		if after != nil {
			return after(ctx)
		}

		return nil
	})
}

func setup(ctx context.Context, db *pgxdb.DB) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS users (
		id    BIGSERIAL PRIMARY KEY,
		email TEXT NOT NULL
	)`); err != nil {
		return err
	}

	_, err := db.Exec(ctx, schema)

	return err
}
