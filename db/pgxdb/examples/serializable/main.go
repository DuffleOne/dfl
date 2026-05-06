// Example program demonstrating TxSerializableCtx and its built-in retry. A
// few goroutines each run a serializable transaction that reads then
// increments a counter row; the conflicting writes produce SerializationFailure
// errors, which retrySerializable absorbs by re-running the closure on a
// fresh transaction. The final counter value should equal the worker count.
//
// Run:
//
//	DATABASE_URL=postgres://localhost/example go run ./db/pgxdb/examples/serializable
package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/duffleone/dfl/db/pgxdb"
)

const workers = 4

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

	if _, err := db.Exec(ctx, "DROP TABLE IF EXISTS counters"); err != nil {
		log.Fatalf("drop: %v", err)
	}

	if _, err := db.Exec(ctx, `CREATE TABLE counters (id INT PRIMARY KEY, value INT NOT NULL)`); err != nil {
		log.Fatalf("create: %v", err)
	}

	if _, err := db.Exec(ctx, "INSERT INTO counters (id, value) VALUES (1, 0)"); err != nil {
		log.Fatalf("seed: %v", err)
	}

	var wg sync.WaitGroup

	for i := range workers {
		wg.Go(func() {
			err := db.TxSerializableCtx(ctx, func(ctx context.Context) error {
				q := pgxdb.GetQuerier(ctx, db)

				var n int
				if err := q.QueryRow(ctx,
					"SELECT value FROM counters WHERE id = 1",
				).Scan(&n); err != nil {
					return err
				}

				_, err := q.Exec(ctx,
					"UPDATE counters SET value = $1 WHERE id = 1", n+1,
				)

				return err
			})
			if err != nil {
				log.Printf("worker %d: %v", i, err)
			}
		})
	}

	wg.Wait()

	var final int
	if err := db.QueryRow(ctx,
		"SELECT value FROM counters WHERE id = 1",
	).Scan(&final); err != nil {
		log.Fatalf("final: %v", err)
	}

	log.Printf("counter = %d (expected %d)", final, workers)
}
