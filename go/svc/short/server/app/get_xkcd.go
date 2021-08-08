package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"dfl/svc/short"

	"github.com/cuvva/cuvva-public-go/lib/ptr"
	"github.com/nishanths/go-xkcd/v2"
)

func (a *App) GetXKCD(ctx context.Context, id string) (*short.Resource, error) {
	var comic xkcd.Comic
	var err error

	switch id {
	case "latest":
		comic, err = a.XKCD.Latest(ctx)
	default:
		number, err := strconv.Atoi(id)
		if err != nil {
			return nil, err
		}

		comic, err = a.XKCD.Get(ctx, number)
	}

	if err != nil {
		return nil, err
	}

	date, err := time.Parse("2006 002", fmt.Sprintf("%d %.3d", comic.Year, comic.Day))
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%d-%s.png", comic.Number, comic.Title)

	return &short.Resource{
		ID:        strconv.Itoa(comic.Number),
		Type:      "xkcd",
		Hash:      ptr.String(fmt.Sprintf("XKCD#%d", comic.Day)),
		Name:      &name,
		Link:      comic.ImageURL,
		MimeType:  ptr.String("image/png"),
		Shortcuts: []string{":xkcd"},
		CreatedAt: date,
	}, nil
}
