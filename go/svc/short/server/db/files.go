package db

import (
	"context"
	"fmt"
	"strings"

	"dfl/svc/short"
)

// NewFile inserts a new file into the database
func (qw *QueryableWrapper) NewFile(ctx context.Context, id, s3, ownerID string, name *string, mimetype string) (*short.Resource, error) {
	b := NewQueryBuilder()

	query, values, err := b.
		Insert("resources").
		Columns("id, type, owner_id, name, link, mime_type").
		Values(id, "file", ownerID, name, s3, mimetype).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(resourceColumns, ","))).
		ToSql()
	if err != nil {
		return nil, err
	}

	return qw.queryOne(ctx, query, values)
}

// NewPendingFile inserts a new pending file into the database
func (qw *QueryableWrapper) NewPendingFile(ctx context.Context, id, s3, ownerID string, name *string, mimetype string) (*short.Resource, error) {
	b := NewQueryBuilder()

	query, values, err := b.
		Insert("resources").
		Columns("id, type, owner_id, name, link, mime_type").
		Values(id, "file", ownerID, name, s3, mimetype).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(resourceColumns, ","))).
		ToSql()
	if err != nil {
		return nil, err
	}

	return qw.queryOne(ctx, query, values)
}
