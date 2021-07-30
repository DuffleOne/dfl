package cachet

import (
	sdk "github.com/andygrunwald/cachet"
)

type Client struct {
	*sdk.Client
}

func (c *Client) ListAllComponents() (components []sdk.Component, err error) {
	res, _, err := c.Components.GetAll(&sdk.ComponentsQueryParams{
		Enabled: true,
	})
	if err != nil {
		return nil, err
	}

	if res.Meta.Pagination.TotalPages == 1 {
		return res.Components, nil
	}

	// TODO(lm): test this
	for i := 1; i < res.Meta.Pagination.TotalPages; i++ {
		res, _, err = c.Components.GetAll(&sdk.ComponentsQueryParams{
			Enabled: true,
			QueryOptions: sdk.QueryOptions{
				Page: i,
			},
		})
		if err != nil {
			return nil, err
		}

		components = append(components, res.Components...)
	}

	return
}
