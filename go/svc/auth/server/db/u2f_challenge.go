package db

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	"github.com/cuvva/cuvva-public-go/lib/ptr"
	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/webauthn"
	"github.com/lib/pq"
)

func (qw *QueryableWrapper) FindU2FChallenge(ctx context.Context, id string) (*webauthn.SessionData, error) {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Select("challenge", "user_id", "allowed_credentials_id", "user_verification").
		From("u2f_challenges").
		Where(sq.Eq{
			"id": id,
		}).
		Where(sq.GtOrEq{
			"expires_at": time.Now(),
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var challenge string
	var userID string
	var allowedCredentialIDs []string
	var userVerification *string

	row := qw.q.QueryRowContext(ctx, query, values...)

	if err := row.Scan(&challenge, &userID, pq.Array(&allowedCredentialIDs), &userVerification); err != nil {
		return nil, err
	}

	var sessionAllowed [][]byte
	var uv protocol.UserVerificationRequirement

	for _, a := range allowedCredentialIDs {
		sessionAllowed = append(sessionAllowed, []byte(a))
	}

	if userVerification != nil {
		uv = protocol.UserVerificationRequirement(*userVerification)
	}

	return &webauthn.SessionData{
		Challenge:            challenge,
		UserID:               []byte(userID),
		AllowedCredentialIDs: sessionAllowed,
		UserVerification:     uv,
	}, nil
}

func (qw *QueryableWrapper) CreateU2FChallenge(ctx context.Context, session *webauthn.SessionData) (string, error) {
	var userVerification *string
	allowedCredentialIDs := []string{}

	for _, ac := range allowedCredentialIDs {
		allowedCredentialIDs = append(allowedCredentialIDs, string(ac))
	}

	if session.UserVerification != "" {
		userVerification = ptr.String(string(session.UserVerification))
	}

	now := time.Now()

	id := ksuid.Generate("u2fchal").String()

	qb := NewQueryBuilder()
	query, values, err := qb.
		Insert("u2f_challenges").
		Columns("id", "challenge", "user_id", "allowed_credentials_id", "user_verification", "created_at", "expires_at").
		Values(
			id,
			session.Challenge,
			string(session.UserID),
			pq.Array(allowedCredentialIDs),
			userVerification,
			now.Format(time.RFC3339),
			now.Add(5*time.Minute).Format(time.RFC3339),
		).
		ToSql()
	if err != nil {
		return "", err
	}

	if _, err := qw.q.ExecContext(ctx, query, values...); err != nil {
		return "", err
	}

	return id, nil
}
