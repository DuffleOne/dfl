package app

import (
	"context"
)

type Cache struct {
	m map[string]int
}

func (a *App) CacheCachet(_ context.Context) error {
	a.Cache = &Cache{
		m: make(map[string]int),
	}

	components, err := a.Cachet.ListAllComponents()
	if err != nil {
		return err
	}

	for _, c := range components {
		a.Cache.m[c.Name] = c.ID
	}

	return nil
}
