package db

import (
	"context"
	"encoding/json"
	"time"

	"dfl/svc/auth"

	sq "github.com/Masterminds/squirrel"
	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	"github.com/duo-labs/webauthn/webauthn"
	"github.com/dvsekhvalnov/jose2go/base64url"
)

func (qw *QueryableWrapper) FindU2FCredential(ctx context.Context, id string) (*auth.U2FCredential, error) {
	return qw.findOneU2FCredential(ctx, sq.Eq{
		"id": id,
	})
}

func (qw *QueryableWrapper) findOneU2FCredential(ctx context.Context, where map[string]interface{}) (*auth.U2FCredential, error) {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Select("id", "name", "user_id", "key_id", "public_key", "attestation_type", "authenticator", "signed_at", "deleted_at").
		From("u2f_credentials").
		Where(where).
		Where(sq.Eq{"deleted_at": nil}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var c auth.U2FCredential

	row := qw.q.QueryRowContext(ctx, query, values...)

	var keyID, pubKey string
	var rawBytes []byte

	if err := row.Scan(&c.ID, &c.Name, &c.UserID, &keyID, &pubKey, &c.Credential.AttestationType, &rawBytes, &c.SignedAt, &c.DeletedAt); err != nil {
		return nil, coerceNotFound(err)
	}

	var id, pub []byte
	var authenticator webauthn.Authenticator

	if id, err = base64url.Decode(keyID); err != nil {
		return nil, err
	}

	if pub, err = base64url.Decode(pubKey); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(rawBytes, &authenticator); err != nil {
		return nil, err
	}

	c.Credential.ID = id
	c.Credential.PublicKey = pub
	c.Credential.Authenticator = authenticator

	return &c, nil
}

func (qw *QueryableWrapper) ListU2FCredentials(ctx context.Context, userID string, includeUnsigned bool) (credentials []auth.U2FCredential, err error) {
	qb := NewQueryBuilder()
	builder := qb.
		Select("id", "name", "user_id", "key_id", "public_key", "attestation_type", "authenticator", "signed_at").
		From("u2f_credentials").
		Where(sq.Eq{"user_id": userID}).
		Where(sq.Eq{"deleted_at": nil})

	if !includeUnsigned {
		builder = builder.Where(sq.NotEq{"signed_at": nil})
	}

	query, values, err := builder.
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := qw.q.QueryContext(ctx, query, values...)
	if err != nil {
		return nil, coerceNotFound(err)
	}

	for rows.Next() {
		var dflID, userID string
		var dflName *string
		var signedAt *time.Time
		var keyID, pubKey, atType string
		var rawBytes []byte

		if err = rows.Scan(&dflID, &dflName, &userID, &keyID, &pubKey, &atType, &rawBytes, &signedAt); err != nil {
			return nil, err
		}

		var id, pub []byte
		var authenticator webauthn.Authenticator

		if id, err = base64url.Decode(keyID); err != nil {
			return
		}

		if pub, err = base64url.Decode(pubKey); err != nil {
			return
		}

		if err := json.Unmarshal(rawBytes, &authenticator); err != nil {
			return nil, err
		}

		credentials = append(credentials, auth.U2FCredential{
			ID:       dflID,
			Name:     dflName,
			UserID:   userID,
			SignedAt: signedAt,
			Credential: webauthn.Credential{
				ID:              id,
				PublicKey:       pub,
				AttestationType: atType,
				Authenticator:   authenticator,
			},
		})
	}

	return
}

func (qw *QueryableWrapper) CreateU2FCredential(ctx context.Context, userID, challengeID string, keyName *string, credential *webauthn.Credential, signedAt *time.Time) (string, error) {
	bytes, err := json.Marshal(credential.Authenticator)
	if err != nil {
		return "", err
	}

	id := ksuid.Generate("u2fkey").String()

	qb := NewQueryBuilder()
	query, values, err := qb.
		Insert("u2f_credentials").
		Columns("id", "user_id", "name", "key_id", "public_key", "attestation_type", "authenticator", "u2f_challenge_id", "signed_at").
		Values(
			id,
			userID,
			keyName,
			base64url.Encode(credential.ID),
			base64url.Encode(credential.PublicKey),
			credential.AttestationType,
			bytes,
			challengeID,
			signedAt,
		).
		ToSql()

	if _, err = qw.q.ExecContext(ctx, query, values...); err != nil {
		return id, err
	}

	return id, nil
}

func (qw *QueryableWrapper) DeleteU2FCredential(ctx context.Context, userID, keyID string) error {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Update("u2f_credentials").
		Set("deleted_at", time.Now()).
		Where(sq.Eq{"id": keyID}).
		Where(sq.Eq{"user_id": userID}).
		Where(sq.Eq{"deleted_at": nil}).
		ToSql()
	if err != nil {
		return err
	}

	if _, err := qw.q.ExecContext(ctx, query, values...); err != nil {
		return err
	}

	return nil
}

func (qw *QueryableWrapper) SignU2FCredential(ctx context.Context, id string) error {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Update("u2f_credentials").
		Set("signed_at", time.Now()).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}

	if _, err := qw.q.ExecContext(ctx, query, values...); err != nil {
		return err
	}

	return nil
}
