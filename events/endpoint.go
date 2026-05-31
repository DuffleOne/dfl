package events

import (
	"context"
	"net/http"
	"strings"

	dflhttp "github.com/duffleone/dfl/http"
)

// RegisterEndpoint exposes a handler for event E over HTTP, the synchronous twin
// of On. It registers POST /events/{segment} on rg, where segment comes from
// E's URLSafeName if it implements URLSafeNamer, or from sanitising EventName
// otherwise. The route is registered at boot, like any http route.
//
// The http router binds the request body into E via the event's json tags (the
// same wire form as the default JSON codec), then the endpoint validates the
// event with the bus validator, calls handler, and returns 204 No Content. Any
// error is projected onto a *dflhttp.ReqError so the http router serialises it:
// validation and decode failures become 400, everything else 500.
//
// Unlike On, this path is fully synchronous: the POST blocks until handler
// returns and the error (if any) is the HTTP response. It does not touch the
// Sink, so it works whether or not anything is subscribed in-process.
func (b *Bus) RegisterEndpoint[E Event](
	rg *dflhttp.Router,
	handler func(context.Context, E) error,
	mw ...dflhttp.Middleware,
) {
	ev, err := zeroEvent[E]()
	if err != nil {
		panic("dflevents: " + err.Error())
	}

	segment := pathSafe(ev.EventName())
	if u, ok := ev.(URLSafeNamer); ok {
		segment = u.URLSafeName()
	}

	rg.Handle(http.MethodPost, "/events/"+segment, func(ctx context.Context, req *E) (*dflhttp.Empty, error) {
		e := *req

		if err := b.validator.Validate(e); err != nil {
			return nil, b.asReqError(err, e.EventName())
		}

		if err := handler(ctx, e); err != nil {
			return nil, b.asReqError(err, e.EventName())
		}

		return &dflhttp.Empty{}, nil
	}, mw...)
}

// asReqError projects an events error onto a *dflhttp.ReqError so the http
// router can serialise it. The original EventError is recorded as a reason for
// errors.As traversal, and its Code drives the HTTP status.
func (b *Bus) asReqError(err error, name string) error {
	eventErr := b.coercer(err)
	if eventErr == nil {
		return nil
	}

	eventErr = eventErr.withEvent(name)

	status := http.StatusInternalServerError
	switch eventErr.Code {
	case "validation_failed", "invalid", "decode_failed":
		status = http.StatusBadRequest
	}

	return dflhttp.New(status, eventErr.Code, dflhttp.M(eventErr.Meta), eventErr)
}

// pathSafe turns an event name into a URL path segment: lowercase, keeping
// [a-z0-9._-] and replacing anything else (notably '/') with '-'. So
// "user.created" stays "user.created" and "Orders/Shipped" becomes
// "orders-shipped".
func pathSafe(name string) string {
	var b strings.Builder

	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	return b.String()
}
