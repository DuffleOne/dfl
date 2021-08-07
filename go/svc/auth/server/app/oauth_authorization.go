package app

import (
	"context"
	"fmt"
	"net/url"
	"time"

	authlib "dfl/lib/auth"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/cuvva/cuvva-public-go/lib/ptr"
	"github.com/cuvva/cuvva-public-go/lib/slicecontains"
	"golang.org/x/crypto/bcrypt"
)

func (a *App) Authorization(ctx context.Context, req *auth.AuthorizeConfirmRequest, user *auth.User) (*auth.AuthorizeConfirmResponse, error) {
	client, err := a.FindClient(ctx, req.ClientID)
	if err != nil {
		return nil, err
	}

	if err := a.CheckLoginValidity(ctx, user); err != nil {
		return nil, err
	}

	if err := a.AuthCodeNoNonceExists(ctx, req.Nonce); err != nil {
		return nil, err
	}

	if req.RedirectURI != nil && !slicecontains.String(client.RedirectURIs, *req.RedirectURI) {
		return nil, cher.New("invalid_redirect_uri", nil)
	}

	if !authlib.Can(req.Scope, user.Scopes) {
		return nil, cher.New(cher.AccessDenied, nil, cher.New("invalid_scopes", nil))
	}

	authCode, expiresAt, err := a.DB.Q.CreateAuthorizationCode(ctx, user.ID, req)
	if err != nil {
		return nil, err
	}

	err = a.DB.Q.ExpireAuthorizationCodes(ctx, ptr.String(authCode))
	if err != nil {
		return nil, err
	}

	rType, params := buildAuthorizeParams(client, req, authCode, expiresAt)

	return &auth.AuthorizeConfirmResponse{
		Type:   rType,
		Params: params,
	}, nil
}

func buildAuthorizeParams(client *auth.Client, req *auth.AuthorizeConfirmRequest, authCode string, expiresAt time.Time) (string, interface{}) {
	switch {
	case len(client.RedirectURIs) == 0:
		expiresIn := expiresAt.Sub(time.Now())

		return "display", &auth.AuthorizeWithoutRedirectParams{
			AuthorizationCode: authCode,
			ExpiresAt:         expiresAt.Format(time.RFC3339),
			ExpiresIn:         int(expiresIn.Seconds()),
			State:             req.State,
		}
	default:
		urlVals := &url.Values{
			"code":  []string{authCode},
			"state": []string{req.State},
		}

		url := fmt.Sprintf("%s?%s", *req.RedirectURI, urlVals.Encode())

		return "redirect", &auth.AuthorizeWithRedirectParams{
			URI: url,
		}
	}
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (a *App) AuthCodeNoNonceExists(ctx context.Context, nonce string) error {
	_, err := a.DB.Q.GetAuthorizationCodeByNonce(ctx, nonce)
	if err != nil {
		if v, ok := err.(cher.E); ok {
			if v.Code == cher.NotFound {
				return nil
			}
		}
	}

	return cher.New("nonce_used", nil)
}
