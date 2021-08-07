package app

import (
	"context"
	"fmt"
	"net/url"

	"dfl/svc/auth"

	"github.com/cuvva/cuvva-public-go/lib/cher"
)

func (a *App) CreateClient(ctx context.Context, req *auth.CreateClientRequest) (*auth.CreateClientResponse, error) {
	var client *auth.Client
	var err error

	uris := []string{}

	for _, uri := range req.RedirectURIs {
		u, err := url.Parse(uri)
		if err != nil {
			return nil, err
		}

		uris = append(uris, fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path))
	}

	client, err = a.DB.Q.GetClientByName(ctx, req.Name)
	if err != nil {
		if v, ok := err.(cher.E); ok && v.Code == cher.NotFound {
			client, err = a.DB.Q.CreateClient(ctx, req.Name, uris)
		}
	}
	if err != nil {
		return nil, err
	}

	return &auth.CreateClientResponse{
		ClientID: client.ID,
	}, nil
}
