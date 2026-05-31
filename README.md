# dfl

Personal monorepo of small Go libraries I reuse across the companies and places I work.

## Packages

### [`http`](./http)

Typed HTTP handlers on top of `net/http`, with structured errors and a pluggable mux.

A handler has shape `func(context.Context, *Req) (*Resp, error)`. The router binds `Req` from path, query, and JSON body via struct tags, calls the handler, then JSON-encodes `Resp` on success or runs the error through a `Coercer` on failure. `Req` and `Resp` are pointer-to-struct so handlers return `(nil, err)` cleanly on the error path. `dflhttp.Empty` (and `*dflhttp.Empty`) covers no-input and no-output routes; an `Empty` resp produces `204 No Content`.

The `Router` wraps any mux that satisfies `MethodFunc(method, pattern, handler)` (chi-style) or `HandleFunc(pattern, handler)` (stdlib `"METHOD /path"`-style). Both `*http.ServeMux` and `*chi.Mux` work directly.

```go
type GetUserReq struct {
    ID string `path:"id"`
}

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func handleGet(_ context.Context, req *GetUserReq) (*User, error) {
    user, ok := store[req.ID]
    if !ok {
        return nil, dflhttp.New(http.StatusNotFound, "user_not_found", dflhttp.M{"id": req.ID})
    }
    return &user, nil
}

func main() {
    r := dflhttp.NewRouter(http.NewServeMux())
    r.Handle(http.MethodGet, "/users/{id}", handleGet)
    log.Fatal(http.ListenAndServe(":8080", r))
}
```

Working examples in [`http/examples/std`](./http/examples/std) and [`http/examples/chi`](./http/examples/chi). [`http/examples/api/widgets.go`](./http/examples/api/widgets.go) shows multi-source request validation (path + query + body) returning a single 400 with field-level errors.

Optional `Coercer` for `samber/oops` errors lives at [`http/oops`](./http/oops).

### [`events`](./events)

A typed event bus, the producer/consumer twin of `http`. You define an event struct, register handlers for it, and emit it; the bus encodes, validates, fans out, and coerces failures into one structured error, so there's no manual marshalling or error plumbing.

An event names itself with `EventName() string`. There are two ways to handle one, both taking the same `func(context.Context, E) error`: `bus.On(handler)` subscribes in-process (delivery is async, and handler errors go to a bus error handler), and `bus.RegisterEndpoint(router, handler)` exposes it over HTTP as `POST /events/{name}` that decodes the event from the JSON body, bridging into the `http` package. `bus.Emit(ctx, e)` publishes and blocks only until the event is committed for delivery.

A `Bus` wraps a pluggable `Sink` the way the `Router` wraps a `Mux`: the in-memory sink ships in the package, and external transports (NATS, a `pgxdb` outbox, a webhook fan-out) drop in behind the same interface. An optional `Validate() error` on the event runs on both publish and delivery, and the validator is pluggable.

```go
type UserCreated struct {
    ID    string `json:"id"`
    Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func welcome(ctx context.Context, e UserCreated) error {
    log.Printf("welcome %s", e.Email)
    return nil
}

func main() {
    bus := events.NewBus(events.NewMemSink())
    bus.On(welcome)
    _ = bus.Emit(context.Background(), UserCreated{ID: "1", Email: "a@b.com"})
}
```

Ready-made cloud transports live in their own modules so the core stays dependency-free: [`events/aws`](./events/aws) (SQS, SNS, EventBridge) and [`events/gcp`](./events/gcp) (Pub/Sub), each with both pull (a receiver loop) and push (an `http.Handler` for the transport's HTTP delivery) where the transport supports it. Plain in-process examples are in [`events/examples`](./events/examples).

A `Plugin` wraps both sides of an event's life, the publish and the deliver, so cross-cutting concerns inject cleanly. The event carries a `Headers` bag that a plugin writes on publish and reads on deliver, and the cloud adapters map it to native message attributes. [`events/otel`](./events/otel) uses this for OpenTelemetry trace propagation: `events.NewBus(sink, events.WithPlugins(otel.New()))` injects trace context on emit and continues the trace in a consumer span on delivery.

### [`db/pgxdb`](./db/pgxdb)

Wrapper around `jackc/pgx/v5`. Transaction shapes (read-only, read-committed, serializable with retry), generic `Get`/`Scalar`/`Select` scanners, and an escape hatch to `*database/sql`. The `Querier` interface is satisfied by both the pool and `pgx.Tx`, so the same helper functions work inside or outside a transaction.

`TxCtx`, `TxReadCtx`, and `TxSerializableCtx` attach the running transaction to the context; helpers can then pull it back out with `GetQuerier(ctx, fallback)`, so a repository function takes a `Querier` once and quietly upgrades to the running tx when called inside a `TxCtx` block. Working examples in [`db/pgxdb/examples/basic`](./db/pgxdb/examples/basic), [`db/pgxdb/examples/tx`](./db/pgxdb/examples/tx), and [`db/pgxdb/examples/serializable`](./db/pgxdb/examples/serializable).
