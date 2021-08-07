package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dfl/svc/short"

	sq "github.com/Masterminds/squirrel"
	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/lib/pq"
)

type ArrayOperation string

const (
	ArrayAdd    ArrayOperation = "array_append"
	ArrayRemove ArrayOperation = "array_remove"
)

// resourceColumns is the set of columns to populate into the struct
var resourceColumns = []string{"id", "type", "serial", "hash", "name", "owner_id", "link", "nsfw", "mime_type", "shortcuts", "created_at", "deleted_at"}

// FindShortcutConflicts returns error if a shortcut is already taken
func (qw *QueryableWrapper) FindShortcutConflicts(ctx context.Context, shortcuts []string) error {
	if len(shortcuts) == 0 {
		return nil
	}

	b := NewQueryBuilder()

	query, values, err := b.
		Select("id").
		From("resources").
		Where("shortcuts @> $1", pq.Array(shortcuts)).
		Limit(1).
		ToSql()

	var id string

	err = coerceNotFound(qw.q.QueryRowContext(ctx, query, values...).Scan(&id))
	if err != nil {
		if v, ok := err.(cher.E); ok && v.Code == cher.NotFound {
			return nil
		}
	}

	return cher.New("shortcut_conflict", cher.M{"shortcuts": shortcuts})
}

// FindResourceByHash retrieves a resource from the database by it's hash
func (qw *QueryableWrapper) FindResourceByHash(ctx context.Context, hash string) (*short.Resource, error) {
	b := NewQueryBuilder()

	query, values, err := b.
		Select(strings.Join(resourceColumns, ",")).
		From("resources").
		Where(sq.Eq{
			"hash": hash,
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	return qw.queryOne(ctx, query, values)
}

// FindResourceByShortcut retrieves a resource from the database by one of it's shortcuts
func (qw *QueryableWrapper) FindResourceByShortcut(ctx context.Context, shortcut string) (*short.Resource, error) {
	b := NewQueryBuilder()

	s := fmt.Sprintf("{%s}", shortcut)

	query, values, err := b.
		Select(strings.Join(resourceColumns, ", ")).
		From("resources").
		Where("shortcuts @> $1::text[]", s).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, err
	}

	return qw.queryOne(ctx, query, values)
}

// FindResourceByName retrieves a resource from the database by an exact name match
func (qw *QueryableWrapper) FindResourceByName(ctx context.Context, name string) (*short.Resource, error) {
	b := NewQueryBuilder()

	query, values, err := b.
		Select(strings.Join(resourceColumns, ",")).
		From("resources").
		Where(sq.Eq{
			"name": name,
		}).
		OrderBy("serial DESC").
		Limit(1).
		ToSql()
	if err != nil {
		return nil, err
	}

	return qw.queryOne(ctx, query, values)
}

func (qw *QueryableWrapper) queryOne(ctx context.Context, query string, values []interface{}) (*short.Resource, error) {
	res := &short.Resource{}

	err := qw.q.QueryRowContext(ctx, query, values...).Scan(
		&res.ID,
		&res.Type,
		&res.Serial,
		&res.Hash,
		&res.Name,
		&res.OwnerID,
		&res.Link,
		&res.NSFW,
		&res.MimeType,
		pq.Array(&res.Shortcuts),
		&res.CreatedAt,
		&res.DeletedAt,
	)

	return res, coerceNotFound(err)
}

// SetNSFW sets a resource NSFW bool
func (qw *QueryableWrapper) SetNSFW(ctx context.Context, resourceID string, state bool) error {
	b := NewQueryBuilder()

	query, values, err := b.
		Update("resources").
		Set("nsfw", state).
		Where(sq.Eq{"id": resourceID}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = qw.q.ExecContext(ctx, query, values...)
	return coerceNotFound(err)
}

// DeleteResource soft-deletes a resource
func (qw *QueryableWrapper) DeleteResource(ctx context.Context, resourceID string) error {
	b := NewQueryBuilder()

	query, values, err := b.
		Update("resources").
		Set("deleted_at", time.Now()).
		Where(sq.Eq{"id": resourceID}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = qw.q.ExecContext(ctx, query, values...)
	return coerceNotFound(err)
}

// SaveHash saves the hash of a resource into the DB
func (qw *QueryableWrapper) SaveHash(ctx context.Context, serial int, hash string) error {
	b := NewQueryBuilder()

	query, values, err := b.
		Update("resources").
		Set("hash", hash).
		Where(sq.Eq{"serial": serial}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = qw.q.ExecContext(ctx, query, values...)
	return coerceNotFound(err)
}

func (qw *QueryableWrapper) ListResources(ctx context.Context, req *short.ListResourcesRequest) ([]*short.Resource, error) {
	b := NewQueryBuilder()

	builder := b.
		Select(strings.Join(resourceColumns, ",")).
		From("resources")

	if !req.IncludeDeleted {
		builder = builder.Where(sq.Eq{"deleted_at": nil})
	}

	if req.OwnerID != nil {
		builder = builder.Where(sq.Eq{"owner_id": *req.OwnerID})
	}

	if req.FilterMime != nil {
		builder = builder.Where(sq.Like{"mime_type": fmt.Sprintf("%s%%", *req.FilterMime)})
	}

	builder = builder.OrderBy("created_at DESC")

	if req.Limit != nil {
		builder = builder.Limit(*req.Limit)
	}

	query, values, err := builder.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := qw.q.QueryContext(ctx, query, values...)
	if err != nil {
		return nil, err
	}

	resources := []*short.Resource{}

	for rows.Next() {
		o := &short.Resource{}

		err := rows.Scan(&o.ID, &o.Type, &o.Serial, &o.Hash, &o.Name, &o.OwnerID, &o.Link, &o.NSFW, &o.MimeType, pq.Array(&o.Shortcuts), &o.CreatedAt, &o.DeletedAt)
		if err != nil {
			return nil, err
		}

		resources = append(resources, o)
	}

	return resources, nil
}

// ListResourcesWithoutHash lists all resources where the hash is not saved
func (qw *QueryableWrapper) ListResourcesWithoutHash(ctx context.Context) ([]*short.ShortFormResource, error) {
	b := NewQueryBuilder()

	query, values, err := b.
		Select("id, serial").
		From("resources").
		Where(sq.Eq{
			"hash": nil,
		}).
		ToSql()

	rows, err := qw.q.QueryContext(ctx, query, values...)
	if err != nil {
		return nil, coerceNotFound(err)
	}

	resources := []*short.ShortFormResource{}

	for rows.Next() {
		o := &short.ShortFormResource{}

		err := rows.Scan(&o.ID, &o.Serial)
		if err != nil {
			return nil, err
		}

		resources = append(resources, o)
	}

	return resources, nil
}

func (qw *QueryableWrapper) ChangeShortcut(ctx context.Context, operation ArrayOperation, resourceID, shortcut string) error {
	b := NewQueryBuilder()

	query, values, err := b.
		Update("resources").
		Set("shortcuts", sq.Expr(fmt.Sprintf("%s(shortcuts, ?)", operation), shortcut)).
		Where(sq.Eq{"id": resourceID}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = qw.q.ExecContext(ctx, query, values...)
	return coerceNotFound(err)
}
