package db

import (
	"context"
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/cuvva/cuvva-public-go/lib/cher"
)

type DB struct {
	db *sql.DB
	Q  *QueryableWrapper
}

type QueryableWrapper struct {
	q queryable
}

type queryable interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// New initializes a new database access object.
func New(db *sql.DB) *DB {
	return &DB{
		db: db,
		Q:  &QueryableWrapper{db},
	}
}

func coerceNotFound(err error) error {
	if err == sql.ErrNoRows {
		return cher.New(cher.NotFound, nil)
	}

	return err
}

func NewQueryBuilder() sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}

func (d *DB) DoTx(ctx context.Context, fn func(qw *QueryableWrapper) error) (err error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic for exception handler
		} else if err != nil {
			tx.Rollback() // dont set err, keep err = error from fn
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(&QueryableWrapper{tx})

	return err
}
