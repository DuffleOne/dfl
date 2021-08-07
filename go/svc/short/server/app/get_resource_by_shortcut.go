package app

import (
	"context"
	"strings"

	"dfl/svc/short"
)

// GetResourceByShortcut returns a resource from a :shortcut
func (a *App) GetResourceByShortcut(ctx context.Context, shortcut string) (*short.Resource, error) {
	if strings.HasPrefix(shortcut, "xkcd") {
		parts := strings.Split(shortcut, "-")

		if len(parts) == 1 {
			return a.GetXKCD(ctx, "latest")
		}

		return a.GetXKCD(ctx, parts[1])
	}

	return a.DB.Q.FindResourceByShortcut(ctx, shortcut)
}
