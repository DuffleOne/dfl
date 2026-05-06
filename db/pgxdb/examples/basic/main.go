// Example program exercising the users helpers against the pool, no
// transactions involved. The pool itself satisfies pgxdb.Querier, so the
// helpers accept it as the fallback Querier and GetQuerier hands it back
// because there's no tx on the context.
//
// Run:
//
//	DATABASE_URL=postgres://localhost/example go run ./db/pgxdb/examples/basic
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

	alice, err := users.Create(ctx, db, "Alice", "alice@example.com")
	if err != nil {
		log.Fatalf("create alice: %v", err)
	}

	log.Printf("created %+v", alice)

	if _, err := users.Create(ctx, db, "Bob", "bob@example.com"); err != nil {
		log.Fatalf("create bob: %v", err)
	}

	if err := users.Rename(ctx, db, alice.ID, "Alice Anderson"); err != nil {
		log.Fatalf("rename: %v", err)
	}

	all, err := users.List(ctx, db)
	if err != nil {
		log.Fatalf("list: %v", err)
	}

	for _, u := range all {
		log.Printf("user %+v", u)
	}
}
