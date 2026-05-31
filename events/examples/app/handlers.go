package main

import (
	"context"
	"log"

	"github.com/duffleone/dfl/events"
	dflhttp "github.com/duffleone/dfl/http"
)

// handlers groups the example's event handlers. Each handler is wired both
// in-process (Subscribe) and over HTTP (MountHTTP), with the same
// func(context.Context, E) error signature in both places.
type handlers struct{}

func (handlers) welcome(_ context.Context, e UserCreated) error {
	log.Printf("welcome %s", e.Email)

	return nil
}

func (handlers) audit(_ context.Context, e UserCreated) error {
	log.Printf("audit: user %s created", e.ID)

	return nil
}

func (handlers) ship(_ context.Context, e OrderShipped) error {
	log.Printf("order %s shipped via %s", e.OrderID, e.Carrier)

	return nil
}

// Subscribe registers the async in-process handlers. user.created fans out to
// both welcome and audit.
func (h handlers) Subscribe(bus *events.Bus) {
	bus.On(h.welcome)
	bus.On(h.audit)
	bus.On(h.ship)
}

// MountHTTP exposes the same handlers as POST /events/{name} endpoints. welcome
// lands at /events/user.created; ship lands at /events/orders-shipped via the
// event's URLSafeName.
func (h handlers) MountHTTP(bus *events.Bus, rg *dflhttp.Router) {
	bus.RegisterEndpoint(rg, h.welcome)
	bus.RegisterEndpoint(rg, h.ship)
}
