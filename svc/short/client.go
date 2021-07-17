package short

import (
	"context"
	"net/http"
	"time"

	"github.com/cuvva/cuvva-public-go/lib/crpc"
	"github.com/cuvva/cuvva-public-go/lib/jsonclient"
)

type client struct {
	*crpc.Client
}

func NewClient(baseURL string, key *string) Service {
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	if key != nil {
		httpClient.Transport = jsonclient.NewAuthenticatedRoundTripper(nil, "Bearer", *key)
	}

	return &client{
		crpc.NewClient(baseURL+"/1", httpClient),
	}
}

func (c *client) AddShortcut(ctx context.Context, req *ChangeShortcutRequest) error {
	return c.Do(ctx, "add_shortcut", "2021-07-17", req, nil)
}

func (c *client) CreateSignedURL(ctx context.Context, req *CreateSignedURLRequest) (res *CreateSignedURLResponse, err error) {
	return res, c.Do(ctx, "create_signed_url", "2021-07-17", req, &res)
}

func (c *client) DeleteResource(ctx context.Context, req *IdentifyResource) error {
	return c.Do(ctx, "delete_resource", "2021-07-17", req, nil)
}

func (c *client) ListResources(ctx context.Context, req *ListResourcesRequest) (res []*Resource, err error) {
	return res, c.Do(ctx, "list_resources", "2021-07-17", req, &res)
}

func (c *client) RemoveShortcut(ctx context.Context, req *ChangeShortcutRequest) error {
	return c.Do(ctx, "remove_shortcut", "2021-07-17", req, nil)
}

func (c *client) SetNSFW(ctx context.Context, req *SetNSFWRequest) error {
	return c.Do(ctx, "set_nsfw", "2021-07-17", req, nil)
}

func (c *client) ShortenURL(ctx context.Context, req *CreateURLRequest) (res *CreateResourceResponse, err error) {
	return res, c.Do(ctx, "shorten_url", "2021-07-17", req, &res)
}

func (c *client) ViewDetails(ctx context.Context, req *IdentifyResource) (res *Resource, err error) {
	return res, c.Do(ctx, "view_details", "2021-07-17", req, &res)
}
