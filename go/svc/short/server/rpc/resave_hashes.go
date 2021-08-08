package rpc

import (
	"context"

	authlib "dfl/lib/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

func (r *RPC) ResaveHashes(ctx context.Context) error {
	authUser := authlib.GetUserContext(ctx)
	if !authUser.Can("short:admin") {
		return cher.New(cher.AccessDenied, nil)
	}

	return r.app.ResaveHashes(ctx)
}
