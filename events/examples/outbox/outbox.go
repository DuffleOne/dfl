package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/duffleone/dfl/db/pgxdb"
	"github.com/duffleone/dfl/events"
)

// schema is the outbox table. published_at doubles as the cursor: NULL means
// the row is still waiting, and the partial index keeps the relay's scan
// proportional to the backlog rather than to the table.
const schema = `
CREATE TABLE IF NOT EXISTS event_outbox (
	id           BIGSERIAL PRIMARY KEY,
	name         TEXT        NOT NULL,
	payload      JSONB       NOT NULL,
	headers      JSONB       NOT NULL DEFAULT '{}',
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS event_outbox_unpublished
	ON event_outbox (id) WHERE published_at IS NULL;
`

// outboxSink is an events.Sink that turns publishing into a database
// write, so an event and the rows it describes commit together or not at
// all. It wraps the sink that actually carries events (MemSink here, SQS
// or Pub/Sub in a real service): Publish writes to the table, Relay moves
// rows onto the inner sink, and Subscribe forwards to it.
type outboxSink struct {
	db   *pgxdb.DB
	next events.Sink
}

var _ events.Sink = (*outboxSink)(nil)

func newOutboxSink(db *pgxdb.DB, next events.Sink) *outboxSink {
	return &outboxSink{db: db, next: next}
}

// Publish inserts env into the outbox through pgxdb.GetQuerier, which is
// the whole trick: inside a TxCtx block it writes on the running
// transaction, so the event lands with the business data or rolls back
// with it; outside one it commits on its own. Returning once the row is
// written satisfies Sink's certain-to-be-delivered contract, since after
// commit the relay will get to it.
func (s *outboxSink) Publish(ctx context.Context, env events.Envelope) error {
	headers, err := json.Marshal(env.Headers)
	if err != nil {
		return err
	}

	_, err = pgxdb.GetQuerier(ctx, s.db).Exec(ctx,
		`INSERT INTO event_outbox (name, payload, headers) VALUES ($1, $2, $3)`,
		env.Name, []byte(env.Payload), headers)

	return err
}

// Subscribe forwards to the inner sink. The outbox is a produce-side concern;
// consumers read from the real transport and never see the table.
func (s *outboxSink) Subscribe(name string, deliver events.HandlerFunc) {
	s.next.Subscribe(name, deliver)
}

// outboxRow is one waiting event. The db tags match the column names.
type outboxRow struct {
	ID      int64             `db:"id"`
	Name    string            `db:"name"`
	Payload json.RawMessage   `db:"payload"`
	Headers map[string]string `db:"headers"`
}

// relayBatch publishes up to limit waiting events and marks them done,
// returning how many moved (0 on error: the tx rolls back as a unit). FOR
// UPDATE SKIP LOCKED lets several relays run on disjoint rows; ORDER BY id
// keeps one relay in emit order. Delivery is at-least-once: a publish
// whose marking commit fails goes out again, so consumers must tolerate
// duplicates. The outbox's trade: duplicates are survivable, phantoms aren't.
func (s *outboxSink) relayBatch(ctx context.Context, limit int) (int, error) {
	var moved int

	err := s.db.TxCtx(ctx, func(ctx context.Context) error {
		moved = 0
		q := pgxdb.GetQuerier(ctx, s.db)

		rows, err := pgxdb.Select[outboxRow](ctx, q, `
			SELECT id, name, payload, headers
			FROM event_outbox
			WHERE published_at IS NULL
			ORDER BY id
			LIMIT $1
			FOR UPDATE SKIP LOCKED`, limit)
		if err != nil {
			return err
		}

		for _, row := range rows {
			env := events.Envelope{Name: row.Name, Payload: row.Payload, Headers: row.Headers}

			if err := s.next.Publish(ctx, env); err != nil {
				return err
			}

			_, err := q.Exec(ctx, `UPDATE event_outbox SET published_at = now() WHERE id = $1`, row.ID)
			if err != nil {
				return err
			}

			moved++
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return moved, nil
}

// Relay drains the outbox, then keeps draining it every interval until ctx
// ends. Run it alongside the service or as its own binary; several copies can
// run at once.
func (s *outboxSink) Relay(ctx context.Context, interval time.Duration, batch int) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		s.drain(ctx, batch)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// drain publishes batches until one comes back short, meaning the backlog is
// clear. An error ends this pass and leaves the rows for the next tick: they
// are still in the table, so nothing is lost by waiting.
func (s *outboxSink) drain(ctx context.Context, batch int) {
	for {
		moved, err := s.relayBatch(ctx, batch)
		if err != nil {
			slog.ErrorContext(ctx, "outbox: relay batch failed", slog.String("error", err.Error()))

			return
		}

		if moved < batch {
			return
		}
	}
}
