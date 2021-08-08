package db

import (
	"context"

	"dfl/svc/auth"

	sq "github.com/Masterminds/squirrel"
	"github.com/cuvva/cuvva-public-go/lib/cher"
)

func (qw *QueryableWrapper) SearchByUsername(ctx context.Context, username string) error {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Select("COUNT(*)").
		From("users").
		Where(sq.ILike{
			"username": username,
		}).
		ToSql()
	if err != nil {
		return err
	}

	var c int

	row := qw.q.QueryRowContext(ctx, query, values...)

	if err := row.Scan(&c); err != nil {
		return err
	}

	if c == 0 {
		return nil
	}

	return cher.New("username_taken", nil)
}

func (qw *QueryableWrapper) findOneUser(ctx context.Context, field, value string) (*auth.User, error) {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Select("u.id, u.username, u.email, u.scopes,u.created_at").
		From("users u").
		Where(sq.Eq{
			field: value,
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var u auth.User

	row := qw.q.QueryRowContext(ctx, query, values...)

	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Scopes, &u.CreatedAt); err != nil {
		return nil, coerceNotFound(err)
	}

	return &u, nil
}

func (qw *QueryableWrapper) FindUser(ctx context.Context, id string) (*auth.User, error) {
	return qw.findOneUser(ctx, "id", id)
}

func (qw *QueryableWrapper) GetUserByName(ctx context.Context, username string) (*auth.User, error) {
	return qw.findOneUser(ctx, "username", username)
}

func (qw *QueryableWrapper) CreateUser(ctx context.Context, id, username, scopes string) (*auth.User, error) {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Insert("users").
		Columns("id", "username", "scopes").
		Values(id, username, scopes).
		Suffix(`RETURNING "id", "username", "email", "scopes", "created_at"`).
		ToSql()
	if err != nil {
		return nil, err
	}

	var u auth.User

	row := qw.q.QueryRowContext(ctx, query, values...)

	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.Scopes, &u.CreatedAt); err != nil {
		return nil, coerceNotFound(err)
	}

	return &u, nil
}
