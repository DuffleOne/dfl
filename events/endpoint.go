package events

import (
	"context"
	"net/http"
	"strings"

	dflhttp "github.com/duffleone/dfl/http"
)

// RegisterEndpoint exposes a handler for E over HTTP, the synchronous twin
// of On: POST /events/{segment}, where segment is E's URLSafeName or its
// sanitised EventName. The router binds the JSON body into E (the default
// codec's wire form), the endpoint validates it, and handler's error is
// the HTTP response: validation and decode failures 400, the rest 500. It
// never touches the Sink, so nothing need be subscribed in-process.
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
	reqErr := dflhttp.New(eventErr.Code, dflhttp.M(eventErr.Meta), eventErr)

	// Bad input from the caller keeps ReqError's 400 default; anything else
	// is the bus failing on our side, so it opts out.
	switch eventErr.Code {
	case "validation_failed", "invalid", "decode_failed":
		return reqErr
	default:
		return reqErr.WithStatus(http.StatusInternalServerError)
	}
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
