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

//go:embed register_confirm.json
var registerConfirmJSON string
var registerConfirmSchema = gojsonschema.NewStringLoader(registerConfirmJSON)

func (r *RPC) RegisterConfirm(ctx context.Context, req *auth.RegisterConfirmRequest) error {
	session, err := r.app.FindU2FChallenge(ctx, req.ChallengeID)
	if err != nil {
		return err
	}

	user := &auth.User{
		ID:       string(session.UserID),
		Username: req.Username,
	}

	waUser, err := r.app.ConvertUserForWA(ctx, user, true)
	if err != nil {
		return err
	}

	if bytes.Compare(session.UserID, waUser.WebAuthnID()) != 0 {
		return cher.New(cher.NotFound, nil) // pretend to not know whats going on
	}

	parsed, err := parseCredentialRegister(req)
	if err != nil {
		return err
	}

	credential, err := r.app.WA.CreateCredential(waUser, *session, parsed)
	if err != nil {
		return err
	}

	if _, err := r.app.Register(ctx, req, credential); err != nil {
		return err
	}

	return nil
}

func parseCredentialRegister(req *auth.RegisterConfirmRequest) (*protocol.ParsedCredentialCreationData, error) {
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
