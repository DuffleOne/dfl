// Package events provides a typed event bus, the producer/consumer twin
// of the http package. An event names itself with EventName, a handler is
// func(ctx, E) error, and Emit publishes the struct directly. On
// subscribes in-process with async delivery; RegisterEndpoint serves the
// same handler at POST /events/{name}. The Bus wraps a Sink as the Router
// wraps a Mux: MemSink by default, cloud transports drop in behind it.
package events

import (
	"context"
	"log/slog"
	"reflect"
	"slices"
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
// the same shape and composition as the http package's Middleware. Middleware
// runs on the consume side; for the produce side see PublishMiddleware.
type Middleware func(next HandlerFunc) HandlerFunc

// PublishFunc publishes an envelope. It's the produce-side analog of
// HandlerFunc: the base PublishFunc hands the envelope to the sink, and
// PublishMiddleware wraps it. The envelope is a pointer so middleware can mutate
// it, for instance to inject trace headers before it's sent.
type PublishFunc func(ctx context.Context, env *Envelope) error

// PublishMiddleware wraps a PublishFunc on the produce side. A Plugin uses it to
// inject context (trace headers, metadata) into the outgoing envelope and to
// observe or time the publish.
type PublishMiddleware func(next PublishFunc) PublishFunc

// Coercer turns any error into an *EventError suitable for the ErrorHandler and
// for RegisterEndpoint's HTTP projection. nil in, nil out. Pluggable so callers
// can teach the bus about their own error hierarchy.
type Coercer func(error) *EventError

// ErrorHandler receives a handler failure from an On subscription. Because On
// delivery is asynchronous, a handler error can't be returned to the Emit
// caller, so it lands here instead. The default logs via slog at error level;
// override with WithErrorHandler.
type ErrorHandler func(ctx context.Context, env Envelope, err *EventError)

// Plugin is a cross-cutting extension installed with WithPlugins, wrapping
// both sides of an event's life: WrapPublish on the produce side and
// WrapDeliver on the consume side, linked by Envelope.Headers. That's how
// OpenTelemetry plugs in, one call wiring injection on publish and
// extraction on deliver; the concrete plugin lives in its own module,
// events/otel. Either method may return next unchanged to opt out.
type Plugin interface {
	WrapPublish(next PublishFunc) PublishFunc
	WrapDeliver(next HandlerFunc) HandlerFunc
}

// PluginFuncs adapts plain middleware into a Plugin. A nil field leaves that
// side unchanged, so a publish-only or deliver-only plugin is a one-liner:
//
//	events.PluginFuncs{Deliver: logEvents}
type PluginFuncs struct {
	Publish PublishMiddleware
	Deliver Middleware
}

// WrapPublish satisfies Plugin.
func (p PluginFuncs) WrapPublish(next PublishFunc) PublishFunc {
	if p.Publish == nil {
		return next
	}

	return p.Publish(next)
}

// WrapDeliver satisfies Plugin.
func (p PluginFuncs) WrapDeliver(next HandlerFunc) HandlerFunc {
	if p.Deliver == nil {
		return next
	}

	return p.Deliver(next)
}

// Bus is the event bus. It wraps a Sink, holds the codec, validator, coercer,
// and error handler, and turns typed On registrations into Envelope-level
// deliver callbacks on the sink. Construct with NewBus, register handlers with
// On (or RegisterEndpoint), then publish with Emit.
type Bus struct {
	sink       Sink
	codec      *Codec
	validator  Validator
	coercer    Coercer
	onError    ErrorHandler
	middleware []Middleware
	publishMW  []PublishMiddleware
}

// Option configures a Bus.
type Option func(*Bus)

// WithCodec sets the Codec used to encode and decode events. Defaults to
// DefaultCodec (JSON).
func WithCodec(c *Codec) Option {
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

// WithPlugins installs plugins: each plugin's WrapDeliver joins the consume
// middleware chain (applied to every On registered afterwards) and its
// WrapPublish the publish chain. Install at construction, before registering
// handlers.
func WithPlugins(plugins ...Plugin) Option {
	return func(b *Bus) {
		for _, p := range plugins {
			b.middleware = append(b.middleware, p.WrapDeliver)
			b.publishMW = append(b.publishMW, p.WrapPublish)
		}
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

// On registers an in-process handler for events of type E; the name is
// derived from E once, and a malformed E (or a codec rejecting it via
// PrepareFor) panics here, not on the first event. Delivery is async on
// the sink's goroutines: decode, validate, then handler. Errors are
// coerced and given to the bus ErrorHandler, and returned to the sink so
// a durable transport can nack; Emit's caller never sees them.
func (b *Bus) On[E Event](handler func(context.Context, E) error, mw ...Middleware) {
	name, err := nameOf[E]()
	if err != nil {
		panic("dflevents: " + err.Error())
	}

	if err := b.codec.PrepareFor[E](); err != nil {
		panic("dflevents: " + err.Error())
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

		coerced := b.coercer(err)
		if coerced == nil {
			return nil
		}

		b.onError(ctx, env, coerced.withEvent(env.Name))

		// Returned to the sink as well as the ErrorHandler: a fire-and-forget
		// sink like MemSink ignores it, but a durable transport can use it to
		// nack and have the message redelivered.
		return coerced
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

	env := Envelope{Name: name, Payload: payload, Headers: map[string]string{}}

	base := PublishFunc(func(ctx context.Context, env *Envelope) error {
		return b.sink.Publish(ctx, *env)
	})

	if err := applyPublish(base, b.publishMW)(ctx, &env); err != nil {
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
	for _, v := range slices.Backward(mw) {
		h = v(h)
	}

	return h
}

func applyPublish(p PublishFunc, mw []PublishMiddleware) PublishFunc {
	for _, v := range slices.Backward(mw) {
		p = v(p)
	}

	return p
}
