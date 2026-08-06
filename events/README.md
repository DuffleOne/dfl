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

## Arriving early

A consumer can beat the write it depends on. The producer emits, the
transport delivers, and the transaction that writes the row the event names
hasn't committed yet, so the handler looks for something that isn't there.
That isn't a failure, it's an ordering, and the answer is to wait and look
again:

```go
bus.Use(events.Retry(events.RetryPolicy{}))

bus.On(func(ctx context.Context, e UserCreated) error {
    user, err := store.Get(ctx, e.ID)
    if errors.Is(err, pgxdb.NotFound) {
        return events.NotFound
    }
    // ...
})
```

The zero policy is three attempts, backing off 50ms then 100ms, retrying
only on not-found and handing the error back to the sink if they run out.
Signal not-found by returning `events.NotFound`, wrapping it, or building
your own with the same code and some meta:

```go
return events.New("not_found", events.M{"user_id": e.ID})
```

Everything is swappable:

```go
bus.Use(events.Retry(events.RetryPolicy{
    Attempts:  5,
    Backoff:   events.ExponentialBackoff(200 * time.Millisecond),
    Retryable: func(err error) bool { return errors.Is(err, pgxdb.NotFound) },
    Exhausted: events.DeadLetter(parkForReview),
}))
```

`Retryable` defaults to `events.IsNotFound` rather than "any error" on
purpose. Retrying a genuine bug just makes it happen three times.

### When the attempts run out

`Exhausted` decides. It's a plain func, so a service with its own idea (park
it in a table, page someone, emit a compensating event) writes one; three
come with the package:

| `Exhausted`               | What it does                                                                          |
| ------------------------- | ------------------------------------------------------------------------------------- |
| `events.Nack` (default)   | Logs, returns the error to the sink, and lets the transport's redrive policy decide     |
| `events.DeadLetter(park)` | Logs, hands the envelope to `park`, and acks. A failing `park` nacks instead of losing it |
| `events.Drop`             | Logs and acks. The event is gone                                                        |

`Drop` suits an advisory event where a lost one costs nothing. `DeadLetter`
suits anything somebody needs to look at. `Nack` is the default because a
durable transport usually has its own answer already, and in-process retries
shouldn't quietly overrule an SQS redrive policy or a Pub/Sub dead-letter
topic.

One constraint: the waiting happens inside the delivery, so it holds the
message's lease. Keep the sum of the backoffs well under an SQS visibility
timeout or a Pub/Sub ack deadline, or the transport redelivers underneath
you and the work runs twice.

### Or don't race at all

Retries make an early arrival survivable. They don't help when the write
never lands, because the transaction rolled back: then the event describes
something that will never exist, and the best available outcome is a
dead-letter queue and somebody's afternoon.

A transactional outbox removes the race instead of absorbing it. `Publish`
writes the event into a table on the caller's transaction and a relay moves
rows onto the real transport after the commit, so a rollback takes the event
with it and a commit guarantees it goes out:

```go
outbox := newOutboxSink(db, sqsSink)   // Publish writes a row; Subscribe forwards
bus := events.NewBus(outbox)

go outbox.Relay(ctx, 100*time.Millisecond, 50)

db.TxCtx(ctx, func(ctx context.Context) error {
    id, err := store.Create(ctx, name, email)
    if err != nil {
        return err
    }

    return bus.Emit(ctx, UserCreated{ID: id, Email: email})
})
```

`Sink` is small enough to sit in front of another one, and
[`pgxdb.GetQuerier`](../db/pgxdb) is what makes the transactional part work:
inside a `TxCtx` block it hands back the running transaction, so the insert
joins it without the sink knowing there was one. The cost is a table, a
relay to run, and at-least-once delivery, since a publish that succeeds
before its marking commit fails goes out twice.

[`examples/outbox`](./examples/outbox) is the whole thing: sink, relay, a
committed write whose event arrives, and a rolled back one whose event never
does.

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
- [`examples/outbox`](./examples/outbox): a transactional outbox sink and relay on `pgxdb`
- [`otel/examples/trace`](./otel/examples/trace): end-to-end tracing with a stdout exporter
