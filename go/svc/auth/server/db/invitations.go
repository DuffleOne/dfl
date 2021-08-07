package db

import (
	"context"
	"strings"
	"time"

	"dfl/svc/auth"

	sq "github.com/Masterminds/squirrel"
	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/cuvva/cuvva-public-go/lib/ksuid"
)

func (qw *QueryableWrapper) GetInvitationByCode(ctx context.Context, code string) (*auth.Invitation, error) {
	return qw.findOneInvitation(ctx, map[string]interface{}{
		"code": code,
	})
}

func (qw *QueryableWrapper) findOneInvitation(ctx context.Context, where map[string]interface{}) (*auth.Invitation, error) {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Select("id", "code", "scopes", "created_by", "created_at", "redeemed_by", "redeemed_at", "expires_at").
		From("invitations").
		Where(where).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, err
	}

	var i auth.Invitation

	row := qw.q.QueryRowContext(ctx, query, values...)

	if err := row.Scan(&i.ID, &i.Code, &i.Scopes, &i.CreatedBy, &i.CreatedAt, &i.RedeemedBy, &i.RedeemedAt, &i.ExpiresAt); err != nil {
		return nil, coerceNotFound(err)
	}

	return &i, nil
}

func (qw *QueryableWrapper) CreateInvitation(ctx context.Context, userId string, req *auth.CreateInviteCodeRequest) (string, *time.Time, error) {
	var inviteCode string
	var expiresAt *string

	if req.Code != nil {
		inviteCode = *req.Code
	} else {
		inviteCode = ksuid.Generate("code").String()
	}

	if req.ExpiresAt != nil {
		expiresAtStr := req.ExpiresAt.Format(time.RFC3339)
		expiresAt = &expiresAtStr
	}

	qb := NewQueryBuilder()
	query, values, err := qb.
		Insert("invitations").
		Columns("id", "code", "scopes", "created_by", "created_at", "expires_at").
		Values(
			ksuid.Generate("invitation").String(),
			inviteCode,
			req.Scopes,
			userId,
			time.Now().Format(time.RFC3339),
			expiresAt,
		).
		ToSql()
	if err != nil {
		return "", nil, err
	}

	if _, err := qw.q.ExecContext(ctx, query, values...); err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return "", nil, cher.New("invite_code_used", nil)
		}

		return "", nil, err
	}

	return inviteCode, req.ExpiresAt, nil
}

func (qw *QueryableWrapper) RedeemInvite(ctx context.Context, invitationID, userID string) error {
	qb := NewQueryBuilder()
	query, values, err := qb.
		Update("invitations").
		Set("redeemed_by", userID).
		Set("redeemed_at", time.Now()).
		Where(sq.Eq{"id": invitationID}).
		ToSql()
	if err != nil {
		return err
	}

	if _, err := qw.q.ExecContext(ctx, query, values...); err != nil {
		return err
	}

	return nil
}
