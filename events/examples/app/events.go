package main

import "github.com/duffleone/dfl/events"

// UserCreated fans out to multiple in-process handlers and is also exposed over
// HTTP. It validates itself: Email is required, checked on both publish and
// delivery.
type UserCreated struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func (e UserCreated) Validate() error {
	if e.Email == "" {
		return events.New("validation_failed", events.M{
			"fields": events.M{"email": "is required"},
		})
	}

	return nil
}

// OrderShipped pins a custom HTTP route via URLSafeName: its bus name is
// "order.shipped" but the endpoint lives at /events/orders-shipped.
type OrderShipped struct {
	OrderID string `json:"order_id"`
	Carrier string `json:"carrier"`
}

func (OrderShipped) EventName() string   { return "order.shipped" }
func (OrderShipped) URLSafeName() string { return "orders-shipped" }
