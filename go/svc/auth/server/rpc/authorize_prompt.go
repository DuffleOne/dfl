package rpc

import (
	"context"
	_ "embed"

	"dfl/svc/auth"

	"github.com/xeipuuv/gojsonschema"
)

//go:embed authorize_prompt.json
var authorizePromptJSON string
var authorizePromptSchema = gojsonschema.NewStringLoader(authorizePromptJSON)

func (r *RPC) AuthorizePrompt(ctx context.Context, req *auth.AuthorizePromptRequest) (*auth.AuthorizePromptResponse, error) {
	user, err := r.app.GetUserByName(ctx, req.Username)
	if err != nil {
		return nil, err
	}

	if err := r.app.CheckLoginValidity(ctx, user); err != nil {
		return nil, err
	}

	waUser, err := r.app.ConvertUserForWA(ctx, user, false)
	if err != nil {
		return nil, err
	}

	options, session, err := r.app.WA.BeginLogin(waUser)
	if err != nil {
		return nil, err
	}

	id, err := r.app.CreateU2FChallenge(ctx, session)
	if err != nil {
		return nil, err
	}

	return &auth.AuthorizePromptResponse{
		ID:        id,
		Challenge: options,
	}, nil
}
