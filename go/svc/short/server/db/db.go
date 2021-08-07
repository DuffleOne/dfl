package db

import (
	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/cuvva/cuvva-public-go/lib/cher"
)

// DB is a wrapper around the PG wrapper for easy function calls
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

// NewQueryBuilder returns a new query builder
func NewQueryBuilder() sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}
