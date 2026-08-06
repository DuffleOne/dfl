# events

A typed event bus, the producer/consumer twin of the [`http`](../http)
package. You define an event struct, register handlers for it, and emit it;
the bus encodes, validates, fans out, and coerces failures into one
structured error, so there's no manual marshalling or error plumbing.

```go
import "github.com/duffleone/dfl/events"
```

Needs Go 1.27 or later: the bus uses generic methods, added in that release.

## The event model

An event is a struct that names itself:

```go
type UserCreated struct {
    ID    string `json:"id"`
    Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }
```

The name is the topic. It lives on the type, so `On`, `Emit`, and
`RegisterEndpoint` all derive it without the string being repeated at call
sites. Events are value types by convention, the opposite of `http`'s
pointer-`Req` rule: handlers return only an error, so there's no
`(nil, err)` ergonomic to win, and a value type means `EventName` is
callable on a zero value, which the bus relies on at registration. Put
`EventName` (and any `Validate` or `URLSafeName`) on a value receiver.

The default wire form is the event's `json`-tagged fields, wrapped in an
`Envelope` with the name and a `Headers` bag (more on headers under
Plugins).

## The bus

```go
bus := events.NewBus(events.NewMemSink())

bus.On(func(ctx context.Context, e UserCreated) error {
    log.Printf("welcome %s", e.Email)

    return nil
})

err := bus.Emit(ctx, UserCreated{ID: "1", Email: "a@b.com"})
```

`Emit` validates the outgoing event, encodes it, and publishes through the
sink. It blocks until the sink confirms the event is committed for delivery,
then returns: a nil error means the event is certain to fire. What it never
returns is a handler outcome.

`On` subscribes in-process. Delivery is asynchronous: handlers run on their
own goroutines, and a handler error goes to the bus `ErrorHandler` (by
default it logs via `slog`; swap it with `WithErrorHandler`). The error is
also returned to the sink, so a durable transport can nack and redeliver.

The same handler function can instead (or additionally) be served over
HTTP:

```go
bus.RegisterEndpoint(router, handler) // POST /events/user.created
```

This is the synchronous path: the POST decodes the event from the JSON body
via the `http` package's binding, validates it, runs the handler, and
returns 204, or a `ReqError` on failure (validation and decode failures map
to 400, everything else 500). It doesn't touch the sink at all. An event can
pin its path segment by implementing `URLSafeName() string`; otherwise the
event name is sanitised into one.

## Validation

The bus runs its `Validator` on both sides: at `Emit`, so a producer can't
publish an invalid event, and at delivery, so a consumer can't act on one
(the message may have come from another process, over a real transport).
The default validator calls the event's own `Validate() error` when it has
one:

```go
func (e UserCreated) Validate() error {
    if e.Email == "" {
        return events.New("validation_failed", events.M{"fields": events.M{"email": "is required"}})
    }

    return nil
}
```

Swap the whole scheme (struct-tag validation, say) with `WithValidator`.

## Errors

`EventError` is the bus analog of `ReqError`: a `code`, the `event` name
(stamped by the bus), a `meta` bag, and internal causes for `errors.Is`
and `errors.As`. Build one with `events.New` or `events.Wrap`.
The `Coercer` (pluggable via `WithCoercer`) projects arbitrary handler
errors onto it, exactly as in `http`.

Where errors surface depends on the path:

- `Emit` returns producer-side failures only: validation, encoding, publish.
- `On` handler failures reach the `ErrorHandler`, and the sink (for nack).
- `RegisterEndpoint` failures become the HTTP response.

## Middleware and plugins

Consume-side middleware wraps delivery, with the same onion composition as
the `http` package:

```go
bus.Use(logging, recoverPanics)      // every On registered after this
bus.On(handler, perHandlerMiddleware)
```

A `Plugin` wraps both sides of an event's life: `WrapPublish` on the way
out, `WrapDeliver` on the way in. The link between them is
`Envelope.Headers`, a string bag that travels with the event; a plugin
writes it on publish and reads it on deliver, and the cloud transports map
it to native message attributes. That's how OpenTelemetry trace context
flows across the transport:

```go
bus := events.NewBus(sink, events.WithPlugins(otel.New()))
```

The OTel plugin lives in its own module, [`otel`](./otel), so the core
stays dependency-free. [`examples/plugin`](./examples/plugin) builds a
plugin from scratch (a correlation id carried from emitter to handler) and
is the place to start for writing one; `events.PluginFuncs` turns a plain
middleware into a one-sided plugin.

## Sinks

The bus wraps a `Sink` the way the http `Router` wraps a mux:

```go
type Sink interface {
    Publish(ctx context.Context, env Envelope) error
    Subscribe(name string, deliver HandlerFunc)
}
```

`MemSink` ships in the package: in-process, asynchronous, fan-out per
subscriber, handlers running on contexts detached from the emitter's (so an
emitter's request ending doesn't cancel a half-done handler). The cloud
transports live in their own modules so the core stays SDK-free:

| Module                          | Transport                                     |
| ------------------------------- | --------------------------------------------- |
| [`events/aws`](./aws)           | SQS (pull), SNS (push), EventBridge (push)    |
| [`events/gcp`](./gcp)           | Pub/Sub, both pull and push                   |

### Writing a sink

Two rules make the rest of the machinery honest:

- `Publish` must not report success until the event is certain to be
  delivered: broker ack for a durable transport, subscribers scheduled for
  an in-memory one. `Emit`'s "nil means it will fire" contract rests on
  this.
- `Subscribe` is a boot-time call, like route registration. The deliver
  callback's error return is the nack signal: the bus has already told the
  `ErrorHandler`, so a durable sink's only job on error is to leave the
  message for redelivery.

Embed `events.Dispatcher` for the `Subscribe` half: it's a synchronous
registry whose `Dispatch(ctx, env)` runs every handler for the envelope's
name and joins their errors, which is exactly what a receive loop or push
endpoint wants for its ack decision. The SQS and Pub/Sub sinks are small
worked examples of the pattern.

The conformance suite in `internal/bustest` runs the transport-agnostic
behaviour matrix (delivery, fan-out, two-sided validation, error routing,
middleware) against any bus factory; sinks that live under `events/` run it
directly from their tests, as `mem_test.go` does.

## Examples

- [`examples/basic`](./examples/basic): the smallest round trip
- [`examples/app`](./examples/app): the same handlers wired in-process and over HTTP
- [`examples/middleware`](./examples/middleware): logging and recovery middleware, a custom validator, a custom error handler
- [`examples/plugin`](./examples/plugin): a from-scratch plugin using envelope headers
- [`otel/examples/trace`](./otel/examples/trace): end-to-end tracing with a stdout exporter
