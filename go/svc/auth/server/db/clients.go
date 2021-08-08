package db

import (
	"context"

	"dfl/svc/auth"

	sq "github.com/Masterminds/squirrel"
	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	"github.com/lib/pq"
)

func (qw *QueryableWrapper) findOneClient(ctx context.Context, where map[string]interface{}) (*auth.Client, error) {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Select("id", "name", "redirect_uris", "created_at").
		From("clients").
		Where(where).
		ToSql()
	if err != nil {
		return nil, err
	}

	c := &auth.Client{}

	row := qw.q.QueryRowContext(ctx, query, values...)

	if err := row.Scan(&c.ID, &c.Name, pq.Array(&c.RedirectURIs), &c.CreatedAt); err != nil {
		return nil, coerceNotFound(err)
	}

	return c, nil
}

func (qw *QueryableWrapper) FindClient(ctx context.Context, id string) (*auth.Client, error) {
	return qw.findOneClient(ctx, sq.Eq{
		"id": id,
	})
}

func (qw *QueryableWrapper) GetClientByName(ctx context.Context, name string) (*auth.Client, error) {
	return qw.findOneClient(ctx, sq.Eq{
		"name": name,
	})
}

func (qw *QueryableWrapper) CreateClient(ctx context.Context, name string, uris []string) (*auth.Client, error) {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Insert("clients").
		Columns("id", "name", "redirect_uris").
		Values(ksuid.Generate("client").String(), name, pq.Array(uris)).
		Suffix(`RETURNING "id", "name", "redirect_uris", "created_at"`).
		ToSql()
	if err != nil {
		return nil, err
	}

	c := &auth.Client{}

	row := qw.q.QueryRowContext(ctx, query, values...)

	if err := row.Scan(&c.ID, &c.Name, pq.Array(&c.RedirectURIs), &c.CreatedAt); err != nil {
		return nil, coerceNotFound(err)
	}

	return c, nil
}
