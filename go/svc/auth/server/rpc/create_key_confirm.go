package rpc

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"

	authlib "dfl/lib/auth"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/duo-labs/webauthn/protocol"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed create_key_confirm.json
var createKeyConfirmJSON string
var createKeyConfirmSchema = gojsonschema.NewStringLoader(createKeyConfirmJSON)

func (r *RPC) CreateKeyConfirm(ctx context.Context, req *auth.CreateKeyConfirmRequest) error {
	authUser := authlib.GetUserContext(ctx)
	switch {
	case authUser.ID == req.UserID && authUser.Can("auth:login"):
	case authUser.ID != req.UserID && authUser.Can("auth:create_keys"):
	default:
		return cher.New(cher.AccessDenied, nil)
	}

	user, err := r.app.FindUser(ctx, req.UserID)
	if err != nil {
		return err
	}

	if err := r.app.CheckLoginValidity(ctx, user); err != nil {
		return err
	}

	waUser, err := r.app.ConvertUserForWA(ctx, user, true)
	if err != nil {
		return err
	}

	session, err := r.app.FindU2FChallenge(ctx, req.ChallengeID)
	if err != nil {
		return err
	}

	if bytes.Compare(session.UserID, waUser.WebAuthnID()) != 0 {
		return cher.New(cher.NotFound, nil) // pretend to not know whats going on
	}

	parsed, err := parseCredentialCreateKey(req)
	if err != nil {
		return err
	}

	credential, err := r.app.WA.CreateCredential(waUser, *session, parsed)
	if err != nil {
		return err
	}

	if _, err := r.app.CreateU2FCredential(ctx, user.ID, req.ChallengeID, req.KeyName, credential, nil); err != nil {
		return err
	}

	return nil
}

func parseCredentialCreateKey(req *auth.CreateKeyConfirmRequest) (*protocol.ParsedCredentialCreationData, error) {
	mm := map[string]interface{}{
		"id":    req.WebAuthn.ID,
		"rawId": req.WebAuthn.RawID,
		"type":  req.WebAuthn.Type,
		"response": map[string]interface{}{
			"attestationObject": req.WebAuthn.Response.AttestationObject,
			"clientDataJSON":    req.WebAuthn.Response.ClientDataJSON,
		},
	}

	rawBytes, err := json.Marshal(mm)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(rawBytes)

	parsed, err := protocol.ParseCredentialCreationResponseBody(reader)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}
