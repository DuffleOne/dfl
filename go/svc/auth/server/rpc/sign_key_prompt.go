package rpc

import (
	"context"
	_ "embed"

	authlib "dfl/lib/auth"
	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed sign_key_prompt.json
var signKeyPromptJSON string
var signKeyPromptSchema = gojsonschema.NewStringLoader(signKeyPromptJSON)

func (r *RPC) SignKeyPrompt(ctx context.Context, req *auth.SignKeyPromptRequest) (*auth.SignKeyPromptResponse, error) {
	authUser := authlib.GetUserContext(ctx)
	if authUser.ID != req.UserID {
		return nil, cher.New(cher.AccessDenied, nil)
	}

	user, err := r.app.FindUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	if err := r.app.CanSign(ctx, user.ID, req.KeyToSign); err != nil {
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

	return &auth.SignKeyPromptResponse{
		ID:        id,
		Challenge: options,
	}, nil
}
