// Example program showing how to use TxCtx with a repository whose methods
// don't take a pgxdb.Querier argument. UsersRepo (in repo.go) holds a
// *pgxdb.DB and routes each call through pgxdb.GetQuerier internally. The
// same usersRepo.Create call runs against the running transaction when
// invoked inside a TxCtx closure, and against the pool otherwise, without
// the call site changing shape.
//
// Run:
//
//	DATABASE_URL=postgres://localhost/example go run ./db/pgxdb/examples/tx
package main

import (
	"context"
	"log"
	"os"

	"github.com/duffleone/dfl/db/pgxdb"
	"github.com/duffleone/dfl/db/pgxdb/examples/users"
)

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

	if err := users.Reset(ctx, db); err != nil {
		log.Fatalf("reset: %v", err)
	}

	usersRepo := NewUsersRepo(db)

	// Insert a pair atomically. Both Creates run against the same tx because
	// TxCtx puts it on ctx and UsersRepo.Create's GetQuerier picks it up.

	err = db.TxCtx(ctx, func(ctx context.Context) error {
		if _, err := usersRepo.Create(ctx, "Alice", "alice@example.com"); err != nil {
			return err
		}

		_, err := usersRepo.Create(ctx, "Bob", "bob@example.com")

		return err
	})
	if err != nil {
		log.Fatalf("pair create: %v", err)
	}
}
