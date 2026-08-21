// Example program: the users helpers against the pool alone. The pool
// satisfies pgxdb.Querier, so GetQuerier hands it back, no tx in sight.
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

	// A second alice trips the unique index on email. The classified error
	// answers to IsUniqueViolation by constraint name, no SQLSTATEs in sight.
	_, err = users.Create(ctx, db, "Alice again", "alice@example.com")
	if !pgxdb.IsUniqueViolation(err, "users_email_key") {
		log.Fatalf("duplicate alice: expected a unique violation, got %v", err)
	}

	log.Printf("duplicate email refused by %s", pgxdb.ConstraintName(err))

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
