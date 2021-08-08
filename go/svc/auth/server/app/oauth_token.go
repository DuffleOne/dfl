package app

import (
	"context"
	"crypto/sha256"
	"strings"
	"time"

	authlib "dfl/lib/auth"
	"dfl/svc/auth"
	"dfl/svc/auth/server/db"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	"github.com/dvsekhvalnov/jose2go/base64url"
	"github.com/golang-jwt/jwt"
)

const defaultExpiry = 365 * 24 * time.Hour // 365 days
const authExpiry = 1 * time.Hour

func (a *App) Token(ctx context.Context, req *auth.TokenRequest) (*auth.TokenResponse, error) {
	client, err := a.DB.Q.FindClient(ctx, req.ClientID)
	if err != nil {
		return nil, err
	}

	switch req.GrantType {
	case "authorization_code":
	default:
		return nil, cher.New("unsupported_grant_type", nil)
	}

	ac, err := a.DB.Q.FindAuthorizationCode(ctx, req.Code)
	if err != nil {
		return nil, err
	}

	if time.Now().After(ac.ExpiresAt) {
		return nil, cher.New("code_expired", nil)
	}

	user, err := a.DB.Q.FindUser(ctx, ac.UserID)
	if err != nil {
		return nil, err
	}

	if err := a.CheckLoginValidity(ctx, user); err != nil {
		return nil, err
	}

	if !authlib.Can(ac.Scope, user.Scopes) {
		return nil, cher.New(cher.AccessDenied, nil)
	}

	switch ac.CodeChallengeMethod {
	case "S256":
		h := sha256.New()
		h.Write([]byte(req.CodeVerifier))
		compare := base64url.Encode(h.Sum(nil))

		if compare != ac.CodeChallenge {
			return nil, cher.New("code_challenge_failed", nil)
		}
	default:
		return nil, cher.New("unsupported_challenge_method", nil)
	}

	expiresAt := time.Now().Add(calculateExpiryAt(ac.Scope))
	atID := ksuid.Generate("accesstoken").String()

	token := jwt.NewWithClaims(jwt.SigningMethodES384, authlib.DFLClaims{
		Version:  "01",
		Scopes:   ac.Scope,
		Username: user.Username,
		StandardClaims: jwt.StandardClaims{
			Id:        atID,
			ExpiresAt: expiresAt.Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    a.JWTIssuer,
			NotBefore: time.Now().Add(-1 * time.Second).Unix(),
			Subject:   user.ID,
			Audience:  client.ID,
		},
	})

	tokenString, err := token.SignedString(a.SK.Private())
	if err != nil {
		return nil, err
	}

	err = a.DB.Q.CreateAccessToken(ctx, &db.AccessToken{
		ID:                atID,
		Token:             tokenString,
		AuthorizationCode: ac.ID,
		ExpiresAt:         expiresAt,
	})
	if err != nil {
		return nil, err
	}

	return &auth.TokenResponse{
		UserID:      user.ID,
		AccessToken: tokenString,
		TokenType:   "Bearer",
		Expires:     int(expiresAt.Sub(time.Now()).Seconds()),
	}, nil
}

func calculateExpiryAt(scope string) time.Duration {
	if strings.Contains(scope, "auth:") {
		return authExpiry
	}

	return defaultExpiry
}
