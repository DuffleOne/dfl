package html

import (
	"encoding/json"
	"net/http"
	"strings"

	"dfl/lib/rpc"
	"dfl/svc/auth/server/app"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/cuvva/cuvva-public-go/lib/ptr"
	"github.com/cuvva/cuvva-public-go/lib/slicecontains"
)

func Authorize(a *app.App, w http.ResponseWriter, r *http.Request) error {
	params, err := parseAuthorizeParams(r)
	if err != nil {
		return err
	}

	client, err := a.FindClient(r.Context(), params.ClientID)
	if err != nil {
		return err
	}

	if err := a.AuthCodeNoNonceExists(r.Context(), params.Nonce); err != nil {
		return err
	}

	if params.RedirectURI != nil && !slicecontains.String(client.RedirectURIs, *params.RedirectURI) {
		return cher.New("invalid_redirect_uri", nil)
	}

	paramBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return rpc.QuickTemplate(w, map[string]interface{}{
		"title":       "Authenticate",
		"client_name": client.Name,
		"params":      string(paramBytes),
		"scopes":      strings.Fields(params.Scope),
	}, []string{
		"./resources/auth/authorize.html",
		"./resources/auth/layouts/root.html",
	})
}

func parseAuthorizeParams(r *http.Request) (*params, error) {
	var redirectURI *string

	if v, ok := r.URL.Query()["redirect_uri"]; ok {
		redirectURI = ptr.String(v[0])
	}

	p := &params{
		ResponseType:        r.URL.Query().Get("response_type"),
		RedirectURI:         redirectURI,
		ClientID:            r.URL.Query().Get("client_id"),
		Scope:               r.URL.Query().Get("scope"),
		State:               r.URL.Query().Get("state"),
		Nonce:               r.URL.Query().Get("nonce"),
		CodeChallenge:       r.URL.Query().Get("code_challenge"),
		CodeChallengeMethod: r.URL.Query().Get("code_challenge_method"),
	}

	if !p.validate() {
		return nil, cher.New("invalid_input", cher.M{
			"params": p,
		})
	}

	return p, nil
}

type params struct {
	ResponseType        string  `json:"response_type"`
	RedirectURI         *string `json:"redirect_uri"`
	ClientID            string  `json:"client_id"`
	Scope               string  `json:"scope"`
	State               string  `json:"state"`
	Nonce               string  `json:"nonce"`
	CodeChallenge       string  `json:"code_challenge"`
	CodeChallengeMethod string  `json:"code_challenge_method"`
}

func (p params) validate() bool {
	if p.ResponseType == "" {
		return false
	}
	if !slicecontains.String([]string{"code"}, p.ResponseType) {
		return false
	}
	if p.RedirectURI != nil && *p.RedirectURI == "" {
		return false
	}
	if p.ClientID == "" {
		return false
	}
	if p.Scope == "" {
		return false
	}
	if p.State == "" {
		return false
	}
	if p.Nonce == "" {
		return false
	}
	if p.CodeChallenge == "" {
		return false
	}
	if p.CodeChallengeMethod == "" {
		return false
	}
	if !slicecontains.String([]string{"S256"}, p.CodeChallengeMethod) {
		return false
	}
	return true
}
