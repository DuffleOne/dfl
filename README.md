# dfl

Personal monorepo of small Go libraries I reuse across the companies and
places I work.

Three libraries, one shape. Each wraps a thin, typed layer around a thing Go
already does well (`net/http`, message transports, `pgx`), keeps the wire
plumbing out of application code, and hides its backend behind a small
interface so implementations swap without the application noticing:

| Package                            | What it is                                        | Backend interface |
| ---------------------------------- | ------------------------------------------------- | ----------------- |
| [`http`](./http)                   | Typed HTTP handlers with structured errors        | `Mux`             |
| [`http/oops`](./http/oops)         | Error coercer for `samber/oops`                   |                   |
| [`events`](./events)               | Typed event bus, async in-process or over HTTP    | `Sink`            |
| [`events/aws`](./events/aws)       | SQS, SNS, and EventBridge transports (own module) |                   |
| [`events/gcp`](./events/gcp)       | Pub/Sub transport (own module)                    |                   |
| [`events/otel`](./events/otel)     | OpenTelemetry tracing plugin (own module)         |                   |
| [`db/pgxdb`](./db/pgxdb)           | `pgx/v5` wrapper: tx shapes, generic scanning     | `Querier`         |

Every package README is a full guide; this page is the tour.
[docs/building-a-service.md](./docs/building-a-service.md) walks through
wiring all three into one service.

Needs Go 1.27 or later: the typed APIs are generic methods, added in that
release. The cloud transports are separate modules so the core has no SDK
dependencies; `go get` them only where they're used.

## http

A handler has shape `func(context.Context, *Req) (*Resp, error)`. The router
binds `Req` from path, query, and JSON body via struct tags, calls the
handler, then JSON-encodes `Resp` on success or writes a structured error on
failure. `Req` and `Resp` are pointer-to-struct so the error path is just
`(nil, err)`; `*dflhttp.Empty` covers no-input and no-output routes (a 204).

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

The `Router` wraps any mux with chi-style `MethodFunc` or stdlib-style
`HandleFunc` registration; both `*http.ServeMux` and `*chi.Mux` work
directly, verified by one conformance suite.

Errors are layered: handlers return `*ReqError` (`{code, status_code,
meta}` on the wire), a pluggable `Coercer` maps your own error hierarchy
onto that shape, and `WithErrorWriter` hands the whole error response over
when the wire shape itself is yours rather than dfl's. The
[http guide](./http/README.md) covers all three layers;
[`http/examples/errorwriter`](./http/examples/errorwriter) is a service
emitting its own error envelope verbatim while dfl still does the routing.

## events

The producer/consumer twin of `http`. An event names itself with
`EventName()`; handlers are `func(context.Context, E) error`. `bus.On`
subscribes in-process (async, errors to the bus error handler),
`bus.RegisterEndpoint` serves the same handler as `POST /events/{name}`
through the http router, and `bus.Emit` publishes, returning once the event
is committed for delivery.

```go
type UserCreated struct {
    ID    string `json:"id"`
    Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func main() {
    bus := events.NewBus(events.NewMemSink())
    bus.On(func(ctx context.Context, e UserCreated) error {
        log.Printf("welcome %s", e.Email)
        return nil
    })
    _ = bus.Emit(context.Background(), UserCreated{ID: "1", Email: "a@b.com"})
}
```

The bus validates events on both publish and delivery, and a `Plugin` can
wrap both sides of the trip, carrying metadata in envelope headers that the
cloud transports map to native message attributes. That's how
[`events/otel`](./events/otel) flows one trace from emitter to handler:
`events.NewBus(sink, events.WithPlugins(otel.New()))`. Swap `MemSink` for
[`events/aws`](./events/aws) or [`events/gcp`](./events/gcp) and the
application code doesn't change. The [events guide](./events/README.md)
covers the model, plugins, and writing a sink of your own.

## db/pgxdb

Transaction shapes (`Tx`, `TxRead`, `TxSerializable` with retry), generic
`Get`/`Scalar`/`Select` scanners, and an escape hatch to `database/sql`. The
`Querier` interface is satisfied by both the pool and `pgx.Tx`, and the
`TxCtx` family carries the running transaction on the context, so a
repository method written once against `GetQuerier(ctx, fallback)` joins the
ambient transaction when there is one and uses the pool when there isn't:

```go
db.TxCtx(ctx, func(ctx context.Context) error {
    if _, err := repo.Create(ctx, "Alice", "alice@example.com"); err != nil {
        return err
    }
    _, err := repo.Create(ctx, "Bob", "bob@example.com")
    return err // one transaction, or neither insert
})
```

The [pgxdb guide](./db/pgxdb/README.md) covers scanning, transaction
semantics, and the repository pattern.

## Development

```
task test   # go test for every module
task lint   # golangci-lint + staticcheck for every module
```

CI runs the same via [.github/workflows/ci.yml](./.github/workflows/ci.yml).
The repo is four Go modules (root plus the three cloud transports), and both
router and bus ship conformance suites (`http/internal/routertest`,
`events/internal/bustest`) that run the full behaviour matrix against every
backend, so a new mux or sink starts from a passing spec.
