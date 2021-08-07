package rpc

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"

	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/duo-labs/webauthn/protocol"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed authorize_confirm.json
var authorizeConfirmJSON string
var authorizeConfirmSchema = gojsonschema.NewStringLoader(authorizeConfirmJSON)

func (r *RPC) AuthorizeConfirm(ctx context.Context, req *auth.AuthorizeConfirmRequest) (*auth.AuthorizeConfirmResponse, error) {
	user, err := r.app.GetUserByName(ctx, req.Username)
	if err != nil {
		return nil, err
	}

	waUser, err := r.app.ConvertUserForWA(ctx, user, false)
	if err != nil {
		return nil, err
	}

	session, err := r.app.FindU2FChallenge(ctx, req.ChallengeID)
	if err != nil {
		return nil, err
	}

	if bytes.Compare(session.UserID, waUser.WebAuthnID()) != 0 {
		return nil, cher.New(cher.NotFound, nil) // pretend to not know whats going on
	}

	parsed, err := parseCredentialLogin(req)
	if err != nil {
		return nil, err
	}

	if _, err := r.app.WA.ValidateLogin(waUser, *session, parsed); err != nil {
		return nil, err
	}

	return r.app.Authorization(ctx, req, user)
}

func parseCredentialLogin(req *auth.AuthorizeConfirmRequest) (*protocol.ParsedCredentialAssertionData, error) {
	mm := map[string]interface{}{
		"id":    req.WebAuthn.ID,
		"rawId": req.WebAuthn.RawID,
		"type":  req.WebAuthn.Type,
		"response": map[string]interface{}{
			"authenticatorData": req.WebAuthn.Response.AuthenticatorData,
			"clientDataJSON":    req.WebAuthn.Response.ClientDataJSON,
			"signature":         req.WebAuthn.Response.Signature,
			"userHandle":        req.WebAuthn.Response.UserHandle,
		},
	}

	rawBytes, err := json.Marshal(mm)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(rawBytes)

	parsed, err := protocol.ParseCredentialRequestResponseBody(reader)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}
