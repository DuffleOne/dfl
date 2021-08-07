package db

import (
	"context"
	"time"

	"dfl/svc/auth"

	sq "github.com/Masterminds/squirrel"
	"github.com/cuvva/cuvva-public-go/lib/ksuid"
)

func (qw *QueryableWrapper) GetAuthorizationCodeByNonce(ctx context.Context, nonce string) (*auth.AuthorizationCode, error) {
	return qw.findOneAuthorizationCode(ctx, sq.Eq{
		"nonce": nonce,
	})
}

func (qw *QueryableWrapper) FindAuthorizationCode(ctx context.Context, id string) (*auth.AuthorizationCode, error) {
	return qw.findOneAuthorizationCode(ctx, sq.Eq{
		"id": id,
	})
}

func (qw *QueryableWrapper) findOneAuthorizationCode(ctx context.Context, where map[string]interface{}) (*auth.AuthorizationCode, error) {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Select("id", "client_id", "user_id", "state", "nonce", "code_challenge_method", "code_challenge", "scope", "response_type", "created_at", "expires_at").
		From("authorization_codes").
		Where(where).
		ToSql()
	if err != nil {
		return nil, err
	}

	ac := &auth.AuthorizationCode{}

	row := qw.q.QueryRowContext(ctx, query, values...)

	err = row.Scan(&ac.ID, &ac.ClientID, &ac.UserID, &ac.State, &ac.Nonce, &ac.CodeChallengeMethod, &ac.CodeChallenge, &ac.Scope, &ac.ResponseType, &ac.CreatedAt, &ac.ExpiresAt)
	if err != nil {
		return nil, coerceNotFound(err)
	}

	return ac, nil
}

func (qw *QueryableWrapper) ExpireAuthorizationCodes(ctx context.Context, skip *string) error {
	qb := NewQueryBuilder()
	builder := qb.
		Update("authorization_codes").
		Set("expires_at", time.Now().Add(-1*time.Second))

	if skip != nil {
		builder = builder.Where(sq.NotEq{
			"id": *skip,
		})
	}

	query, values, err := builder.ToSql()
	if err != nil {
		return err
	}

	_, err = qw.q.ExecContext(ctx, query, values...)
	return err
}

func (qw *QueryableWrapper) CreateAuthorizationCode(ctx context.Context, userID string, req *auth.AuthorizeConfirmRequest) (string, time.Time, error) {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Insert("authorization_codes").
		Columns(
			"id",
			"client_id",
			"user_id",
			"response_type",
			"state",
			"nonce",
			"code_challenge_method",
			"code_challenge",
			"scope",
			"expires_at",
		).
		Values(
			ksuid.Generate("authtoken").String(),
			req.ClientID,
			userID,
			req.ResponseType,
			req.State,
			req.Nonce,
			req.CodeChallengeMethod,
			req.CodeChallenge,
			req.Scope,
			time.Now().Add(5*time.Minute),
		).
		Suffix("RETURNING \"id\", \"expires_at\"").
		ToSql()
	if err != nil {
		return "", time.Time{}, err
	}

	var at string
	var expires time.Time

	row := qw.q.QueryRowContext(ctx, query, values...)

	if err := row.Scan(&at, &expires); err != nil {
		return "", time.Time{}, err
	}

	return at, expires, nil
}
