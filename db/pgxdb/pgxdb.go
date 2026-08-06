// Package pgxdb wraps a jackc/pgx/v5 connection pool with a small set of
// helpers: transaction shapes (read, read-committed, serializable-with-retry),
// generic Get/Scalar/Select scanners, and an escape hatch to *database/sql.
//
// The Querier interface is satisfied by both *DB and pgx.Tx, so the same
// helper functions work inside or outside a transaction.
package pgxdb

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// NotFound is the sentinel returned by Get and Scalar when a query yields
// zero rows. Aliased to pgx.ErrNoRows so errors.Is matches either name.
var NotFound = pgx.ErrNoRows //nolint:errname // package-local alias, deliberately not "ErrNotFound"

const maxSerializableTxRetries = 3

// Querier is satisfied by *DB and pgx.Tx, so generic helpers work inside or
// outside a transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DB wraps a *pgxpool.Pool with the helpers in this package.
type DB struct {
	pool *pgxpool.Pool
}

var (
	_ Querier = (*DB)(nil)
	_ Querier = (pgx.Tx)(nil)
)

// New opens a pool against connectionURL and pings to verify it's reachable.
func New(ctx context.Context, connectionURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, connectionURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, err
	}

	return &DB{pool: pool}, nil
}

// Close releases the underlying pool.
func (db *DB) Close() {
	db.pool.Close()
}

// Std returns a *sql.DB that shares the underlying pool, plus a cleanup
// function. Use for libraries that only know about database/sql.
func (db *DB) Std() (*sql.DB, func()) {
	stdDB := stdlib.OpenDBFromPool(db.pool)

	return stdDB, func() { _ = stdDB.Close() }
}

// Exec runs a statement that doesn't return rows.
func (db *DB) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return db.pool.Exec(ctx, query, args...)
}

// Query runs a statement and returns the resulting rows.
func (db *DB) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return db.pool.Query(ctx, query, args...)
}

// QueryRow runs a statement expected to return at most one row.
func (db *DB) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return db.pool.QueryRow(ctx, query, args...)
}

// Tx runs f inside a read-committed transaction. f's error rolls back, nil
// commits. Panics roll back and re-panic.
func (db *DB) Tx(ctx context.Context, f func(tx pgx.Tx) error) error {
	return db.tx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, f)
}

// TxRead runs f inside a read-only repeatable-read transaction.
func (db *DB) TxRead(ctx context.Context, f func(tx pgx.Tx) error) error {
	return db.tx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, f)
}

// TxSerializable runs f inside a serializable transaction, retrying on
// SerializationFailure up to maxSerializableTxRetries times. Other errors,
// or a cancelled context, return immediately.
func (db *DB) TxSerializable(ctx context.Context, f func(tx pgx.Tx) error) error {
	opts := pgx.TxOptions{IsoLevel: pgx.Serializable}

	return retrySerializable(ctx, maxSerializableTxRetries, func() error {
		return db.tx(ctx, opts, f)
	})
}

// retrySerializable runs op up to maxRetries+1 times (one initial try plus
// maxRetries retries). It retries only when op returns a serialization
// failure; any other error, or a cancelled context, ends the loop. Between
// attempts it logs a warn through slog.
func retrySerializable(ctx context.Context, maxRetries int, op func() error) error {
	var err error

	for attempt := range maxRetries + 1 {
		err = op()
		if err == nil {
			return nil
		}

		if ctx.Err() != nil || !isSerializationFailure(err) {
			return err
		}

		if attempt == maxRetries {
			break
		}

		slog.WarnContext(ctx, "retrying serializable transaction",
			slog.Int("attempt", attempt+1),
			slog.String("error", err.Error()),
		)
	}

	return err
}

func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == pgerrcode.SerializationFailure
}

// tx is the underlying transaction runner. The named return lets the deferred
// rollback observe an error from Commit too, not just from f.
func (db *DB) tx(ctx context.Context, opts pgx.TxOptions, f func(tx pgx.Tx) error) (err error) {
	var tx pgx.Tx

	tx, err = db.pool.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)

			panic(p)
		}

		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err = f(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Get fetches exactly one row and scans it into a struct T, columns
// matched by `db` tag or lowercased field name. Returns NotFound on zero
// rows and an error on more than one. For a single non-struct value reach
// for Scalar; for many rows, Select.
//
//	user, err := pgxdb.Get[User](ctx, db, `SELECT ... WHERE id = $1`, id)
func Get[T any](ctx context.Context, q Querier, query string, args ...any) (T, error) {
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		var zero T

		return zero, err
	}

	return pgx.CollectOneRow(rows, pgx.RowToStructByName[T])
}

// Scalar fetches exactly one row of one column and returns the scanned
// value; T is anything pgx can scan into. Returns NotFound on zero rows
// and an error on more than one row or column. Use Get for struct rows.
//
//	n, err := pgxdb.Scalar[int](ctx, db, `SELECT count(*) FROM users`)
//	id, err := pgxdb.Scalar[int64](ctx, db, `INSERT ... RETURNING id`, name)
func Scalar[T any](ctx context.Context, q Querier, query string, args ...any) (T, error) {
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		var zero T

		return zero, err
	}

	return pgx.CollectOneRow(rows, pgx.RowTo[T])
}

// Select fetches zero or more rows, scanning each into a struct T under
// the same tag rules as Get; no rows yields an empty slice, not nil or an
// error. For one struct row reach for Get; for a column of scalars loop
// over q.Query directly, Select's row mapper being struct-only.
//
//	all, err := pgxdb.Select[User](ctx, db, `SELECT ... ORDER BY id`)
func Select[T any](ctx context.Context, q Querier, query string, args ...any) ([]T, error) {
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[T])
}
