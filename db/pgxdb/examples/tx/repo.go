package main

import (
	"context"

	"github.com/duffleone/dfl/db/pgxdb"
	"github.com/duffleone/dfl/db/pgxdb/examples/users"
)

// UsersRepo is the same kind of "users repository" as the package-level
// helpers in db/pgxdb/examples/users, but expressed as methods on a struct
// that holds a *pgxdb.DB. The point is that the methods take only their
// real arguments: no pgxdb.Querier parameter to thread through. Each method
// calls pgxdb.GetQuerier(ctx, r.db), so when called inside a TxCtx closure
// it runs against the running transaction, and otherwise against the pool.
type UsersRepo struct {
	db *pgxdb.DB
}

func NewUsersRepo(db *pgxdb.DB) *UsersRepo {
	return &UsersRepo{db: db}
}

// Create inserts a user and returns the row. Note the absence of a Querier
// parameter: GetQuerier picks the right one off ctx, falling back to r.db.
func (r *UsersRepo) Create(ctx context.Context, name, email string) (users.User, error) {
	q := pgxdb.GetQuerier(ctx, r.db)

	return pgxdb.Get[users.User](ctx, q,
		`INSERT INTO users (name, email) VALUES ($1, $2)
		 RETURNING id, name, email`,
		name, email,
	)
}
