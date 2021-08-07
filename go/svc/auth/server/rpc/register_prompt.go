package rpc

import (
	"context"
	_ "embed"

	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/ksuid"
	"github.com/duo-labs/webauthn/protocol"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed register_prompt.json
var registerPromptJSON string
var registerPromptSchema = gojsonschema.NewStringLoader(registerPromptJSON)

func (r *RPC) RegisterPrompt(ctx context.Context, req *auth.RegisterPromptRequest) (*auth.RegisterPromptResponse, error) {
	if _, err := r.app.CheckRegistrationValidity(ctx, req.Username, req.InviteCode); err != nil {
		return nil, err
	}

	user := &auth.User{
		ID:       ksuid.Generate("temp").String(),
		Username: req.Username,
	}

	waUser, err := r.app.ConvertUserForWA(ctx, user, true)
	if err != nil {
		return nil, err
	}

	options, session, err := r.app.WA.BeginRegistration(waUser)

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

	return &auth.RegisterPromptResponse{
		ID:        id,
		Challenge: options,
	}, nil
}
