package db

import (
	"context"
	"time"
)

type AccessToken struct {
	ID                string    `json:"id"`
	Token             string    `json:"token"`
	AuthorizationCode string    `json:"authorization_code"`
	ExpiresAt         time.Time `json:"expires_at"`
}

func (qw *QueryableWrapper) CreateAccessToken(ctx context.Context, token *AccessToken) error {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Insert("access_tokens").
		Columns("id", "token", "authorization_code", "expires_at").
		Values(token.ID, token.Token, token.AuthorizationCode, token.ExpiresAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = qw.q.ExecContext(ctx, query, values...)
	return err
}
