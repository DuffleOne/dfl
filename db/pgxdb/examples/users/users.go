// Package users is a tiny repository used by the pgxdb examples. Each helper
// runs its query through pgxdb.GetQuerier, so the same function works against
// the pool, inside a TxCtx block, or with a caller-supplied pgx.Tx, without
// the call site having to thread a transaction through.
package users

import (
	"context"

	"github.com/duffleone/dfl/db/pgxdb"
)

type User struct {
	ID    int64  `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
}

const Schema = `CREATE TABLE IF NOT EXISTS users (
	id    BIGSERIAL PRIMARY KEY,
	name  TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE
)`

// Reset drops and recreates the table so example runs are repeatable.
func Reset(ctx context.Context, q pgxdb.Querier) error {
	q = pgxdb.GetQuerier(ctx, q)

	if _, err := q.Exec(ctx, "DROP TABLE IF EXISTS users"); err != nil {
		return err
	}

	_, err := q.Exec(ctx, Schema)

	return err
}

func Create(ctx context.Context, q pgxdb.Querier, name, email string) (User, error) {
	q = pgxdb.GetQuerier(ctx, q)

	return pgxdb.Get[User](ctx, q,
		`INSERT INTO users (name, email) VALUES ($1, $2)
		 RETURNING id, name, email`,
		name, email,
	)
}

func Get(ctx context.Context, q pgxdb.Querier, id int64) (User, error) {
	q = pgxdb.GetQuerier(ctx, q)

	return pgxdb.Get[User](ctx, q,
		`SELECT id, name, email FROM users WHERE id = $1`,
		id,
	)
}

func List(ctx context.Context, q pgxdb.Querier) ([]User, error) {
	q = pgxdb.GetQuerier(ctx, q)

	return pgxdb.Select[User](ctx, q,
		`SELECT id, name, email FROM users ORDER BY id`,
	)
}

func Rename(ctx context.Context, q pgxdb.Querier, id int64, name string) error {
	q = pgxdb.GetQuerier(ctx, q)

	_, err := q.Exec(ctx,
		`UPDATE users SET name = $1 WHERE id = $2`,
		name, id,
	)

	return err
}
