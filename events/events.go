// Package events provides a typed event bus, the producer/consumer twin of the
// http package in this module.
//
// You define an event struct that names itself with EventName, register a
// handler of shape func(context.Context, E) error, and call Emit with the
// struct directly. The Bus encodes the event, validates it, and fans it out;
// you never marshal an envelope or wire up binding by hand.
//
// There are two ways to handle an event, both taking the same handler:
//
//   - On subscribes in-process. Delivery is asynchronous: Emit blocks only until
//     the event is committed for delivery, then returns; handlers run on their
//     own goroutines, and a handler error goes to the bus ErrorHandler.
//   - RegisterEndpoint (see endpoint.go) exposes the handler over HTTP as a POST
//     to /events/{name}, decoding the event from the JSON body. It bridges into
//     the http package and runs synchronously.
//
// The Bus wraps a Sink, the transport, the way the http Router wraps a Mux.
// MemSink is the in-process default; external transports drop in behind the
// same interface.
package events

import (
	"context"
	"log/slog"
	"reflect"
)

// M is a key/value bag used for structured metadata, notably on EventError.
type M map[string]any

// Keys returns the keys of m in unspecified order.
func (m M) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	return keys
}

// HandlerFunc is the lower-level handler shape used by middleware and as the
// internal representation of a typed handler after decode/validate has been
// wired up. It operates on an Envelope, before the typed event is known.
type HandlerFunc func(ctx context.Context, env Envelope) error

// Middleware wraps a HandlerFunc. It can run code before or after, short-circuit
// by returning an error without calling next, or transform next's error. It's
// the same shape and composition as the http package's Middleware.
type Middleware func(next HandlerFunc) HandlerFunc

// Coercer turns any error into an *EventError suitable for the ErrorHandler and
// for RegisterEndpoint's HTTP projection. nil in, nil out. Pluggable so callers
// can teach the bus about their own error hierarchy.
type Coercer func(error) *EventError

// ErrorHandler receives a handler failure from an On subscription. Because On
// delivery is asynchronous, a handler error can't be returned to the Emit
// caller, so it lands here instead. The default logs via slog at error level;
// override with WithErrorHandler.
type ErrorHandler func(ctx context.Context, env Envelope, err *EventError)

// Bus is the event bus. It wraps a Sink, holds the codec, validator, coercer,
// and error handler, and turns typed On registrations into Envelope-level
// deliver callbacks on the sink. Construct with NewBus, register handlers with
// On (or RegisterEndpoint), then publish with Emit.
type Bus struct {
	sink       Sink
	codec      Codec
	validator  Validator
	coercer    Coercer
	onError    ErrorHandler
	middleware []Middleware
}

// Option configures a Bus.
type Option func(*Bus)

// WithCodec sets the Codec used to encode and decode events. Defaults to
// DefaultCodec (JSON).
func WithCodec(c Codec) Option {
	return func(b *Bus) {
		b.codec = c
	}
}

// WithCoercer sets the Coercer used to project handler and publish errors onto
// *EventError. Defaults to DefaultCoercer.
func WithCoercer(c Coercer) Option {
	return func(b *Bus) {
		b.coercer = c
	}
}

// WithErrorHandler sets the ErrorHandler that receives async handler failures.
// Defaults to a handler that logs via slog.
func WithErrorHandler(h ErrorHandler) Option {
	return func(b *Bus) {
		b.onError = h
	}
}

// NewBus wraps sink in a Bus configured with the default codec, validator,
// coercer, and error handler, then applies opts.
func NewBus(sink Sink, opts ...Option) *Bus {
	b := &Bus{
		sink:      sink,
		codec:     DefaultCodec,
		validator: DefaultValidator,
		coercer:   DefaultCoercer,
		onError:   defaultErrorHandler,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Use appends middleware to the bus. It applies to On subscriptions registered
// after the Use call.
func (b *Bus) Use(mw ...Middleware) {
	b.middleware = append(b.middleware, mw...)
}

// On registers an in-process handler for events of type E. The event name is
// derived from E once at registration; a malformed E (or a codec that rejects
// the shape via PrepareFor) panics here rather than on the first event.
//
// Delivery is asynchronous. When an event of E's name is published, the sink
// runs the deliver callback on its own goroutine: it decodes the payload into
// E, validates it, and calls handler. A non-nil error (from decode, validation,
// middleware, or the handler) is coerced and passed to the bus ErrorHandler, it
// is not returned to whoever called Emit.
func (b *Bus) On[E Event](handler func(context.Context, E) error, mw ...Middleware) {
	name, err := nameOf[E]()
	if err != nil {
		panic("dflevents: " + err.Error())
	}

	if pre, ok := b.codec.(preparable); ok {
		if err := pre.PrepareFor[E](); err != nil {
			panic("dflevents: " + err.Error())
		}
	}

	base := HandlerFunc(func(ctx context.Context, env Envelope) error {
		e, err := b.codec.Decode[E](env.Payload)
		if err != nil {
			return err
		}

		if err := b.validator.Validate(e); err != nil {
			return err
		}

		return handler(ctx, e)
	})

	wrapped := applyMiddleware(base, combineChain(b.middleware, mw))

	deliver := func(ctx context.Context, env Envelope) error {
		err := wrapped(ctx, env)
		if err == nil {
			return nil
		}

		if coerced := b.coercer(err); coerced != nil {
			b.onError(ctx, env, coerced.withEvent(env.Name))
		}

		return nil
	}

	b.sink.Subscribe(name, deliver)
}

// Emit validates e, encodes it, and publishes it through the sink. It blocks
// until the sink confirms the event is committed for delivery, then returns. The
// only errors it returns are producer-side: validation of the outgoing event,
// encoding, or publish. Handler outcomes are async and reach the ErrorHandler,
// not this return. nil means the event is certain to fire.
func (b *Bus) Emit(ctx context.Context, e Event) error {
	name := e.EventName()

	if err := b.validator.Validate(e); err != nil {
		return b.coerce(err, name)
	}

	payload, err := b.codec.Encode(e)
	if err != nil {
		return b.coerce(err, name)
	}

	if err := b.sink.Publish(ctx, Envelope{Name: name, Payload: payload}); err != nil {
		return b.coerce(err, name)
	}

	return nil
}

// coerce projects err onto an *EventError stamped with the event name, returning
// an untyped nil when the coercer yields nil so callers don't trip over a typed
// nil interface.
func (b *Bus) coerce(err error, name string) error {
	reqErr := b.coercer(err)
	if reqErr == nil {
		return nil
	}

	return reqErr.withEvent(name)
}

// zeroEvent returns a usable zero value of E as an Event. For a pointer E it
// allocates a fresh element so a nil-pointer zero value can't panic when a
// method reads fields; for a value E the zero value is already usable.
func zeroEvent[E Event]() (Event, error) {
	t := reflect.TypeFor[E]()

	if t.Kind() == reflect.Pointer {
		ev, ok := reflect.New(t.Elem()).Interface().(Event)
		if !ok {
			return nil, New("not_an_event", M{"type": t.String()})
		}

		return ev, nil
	}

	var zero E

	return zero, nil
}

// nameOf derives the event name for type E without a caller-supplied value.
func nameOf[E Event]() (string, error) {
	ev, err := zeroEvent[E]()
	if err != nil {
		return "", err
	}

	return ev.EventName(), nil
}

func defaultErrorHandler(ctx context.Context, env Envelope, err *EventError) {
	slog.ErrorContext(ctx, "events: handler failed",
		slog.String("event", env.Name),
		slog.String("error", err.Error()),
	)
}

func combineChain(group, perRoute []Middleware) []Middleware {
	if len(group) == 0 {
		return perRoute
	}

	chain := make([]Middleware, 0, len(group)+len(perRoute))
	chain = append(chain, group...)
	chain = append(chain, perRoute...)

	return chain
}

func applyMiddleware(h HandlerFunc, mw []Middleware) HandlerFunc {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}

	return h
}
