package pgxdb

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// txCtxKey is the unexported context key used by the TxCtx-family methods to
// attach the running transaction to the context.
type txCtxKey struct{}

// contextWithTx returns ctx with tx attached under txCtxKey.
func contextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey{}, tx)
}

// TxFromContext returns the transaction attached by one of the TxCtx-family
// methods, or nil and false if ctx wasn't created inside such a call.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txCtxKey{}).(pgx.Tx)

	return tx, ok
}

// GetQuerier returns the transaction attached to ctx if there is one, or the
// fallback Querier otherwise. Use it in helper functions that may be called
// either inside a TxCtx block (where they should reuse the running tx) or
// outside it (where they should fall back to the pool or a caller-supplied
// tx).
func GetQuerier(ctx context.Context, fallback Querier) Querier {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}

	return fallback
}

// TxCtx is like Tx but attaches the transaction to the context and hands the
// augmented context to f instead of the transaction directly. Inside f, pull
// the transaction back out with TxFromContext.
func (db *DB) TxCtx(ctx context.Context, f func(ctx context.Context) error) error {
	return db.tx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		return f(contextWithTx(ctx, tx))
	})
}

// TxReadCtx is like TxRead but attaches the transaction to the context.
func (db *DB) TxReadCtx(ctx context.Context, f func(ctx context.Context) error) error {
	opts := pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}

	return db.tx(ctx, opts, func(tx pgx.Tx) error {
		return f(contextWithTx(ctx, tx))
	})
}

// TxSerializableCtx is like TxSerializable but attaches the transaction to
// the context. Each retry gets a fresh tx, and therefore a fresh augmented
// context derived from the caller's ctx.
func (db *DB) TxSerializableCtx(ctx context.Context, f func(ctx context.Context) error) error {
	opts := pgx.TxOptions{IsoLevel: pgx.Serializable}

	return retrySerializable(ctx, maxSerializableTxRetries, func() error {
		return db.tx(ctx, opts, func(tx pgx.Tx) error {
			return f(contextWithTx(ctx, tx))
		})
	})
}
