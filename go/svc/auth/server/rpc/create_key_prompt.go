package rpc

import (
	"context"
	_ "embed"

	authlib "dfl/lib/auth"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/duo-labs/webauthn/protocol"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed create_key_prompt.json
var createKeyPromptJSON string
var createKeyPromptSchema = gojsonschema.NewStringLoader(createKeyPromptJSON)

func (r *RPC) CreateKeyPrompt(ctx context.Context, req *auth.CreateKeyPromptRequest) (*auth.CreateKeyPromptResponse, error) {
	authUser := authlib.GetUserContext(ctx)
	switch {
	case authUser.ID == req.UserID && authUser.Can("auth:login"):
	case authUser.ID != req.UserID && authUser.Can("auth:create_keys"):
	default:
		return nil, cher.New(cher.AccessDenied, nil)
	}

	user, err := r.app.FindUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	waUser, err := r.app.ConvertUserForWA(ctx, user, true)
	if err != nil {
		return nil, err
	}

	options, session, err := r.app.WA.BeginRegistration(waUser)
	if err != nil {
		return nil, err
	}

	for _, key := range waUser.Credentials {
		options.Response.CredentialExcludeList = append(options.Response.CredentialExcludeList, protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: key.ID,
		})
	}

	id, err := r.app.CreateU2FChallenge(ctx, session)
	if err != nil {
		return nil, err
	}

	return &auth.CreateKeyPromptResponse{
		ID:        id,
		Challenge: options,
	}, nil
}
