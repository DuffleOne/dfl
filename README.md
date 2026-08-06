# dfl

Small Go libraries I reuse across the places I work. Each one wraps a
thin, typed layer around something Go already does well (`net/http`,
message transports, `pgx`) and hides its backend behind a small
interface, so implementations swap without application code noticing.

Needs Go 1.27+ (the typed APIs are generic methods). The cloud transports
are separate modules, so the core has no SDK dependencies.

| Package                          | What it is                                        |
| -------------------------------- | ------------------------------------------------- |
| [`http`](./http)                 | Typed HTTP handlers with structured errors        |
| [`http/oops`](./http/oops)       | Error coercer for `samber/oops`                   |
| [`http/version`](./http/version) | Versioned endpoints: a handler per API version    |
| [`events`](./events)             | Typed event bus, async in-process or over HTTP    |
| [`events/aws`](./events/aws)     | SQS, SNS, and EventBridge transports (own module) |
| [`events/gcp`](./events/gcp)     | Pub/Sub transport (own module)                    |
| [`events/otel`](./events/otel)   | OpenTelemetry tracing plugin (own module)         |
| [`db/pgxdb`](./db/pgxdb)         | `pgx/v5` wrapper: tx shapes, generic scanning     |

Each package README is the full guide; this page is the tour, and
[docs/building-a-service.md](./docs/building-a-service.md) wires all
three into one service.

## http

Handlers are `func(ctx, *Req) (*Resp, error)`. The router binds `Req`
from path, query, and JSON body via struct tags, then encodes `Resp` or a
structured error. Works over `*http.ServeMux` or `*chi.Mux` unchanged.

```go
type GetUserReq struct {
    ID string `path:"id"`
}

func handleGet(_ context.Context, req *GetUserReq) (*User, error) {
    user, ok := store[req.ID]
    if !ok {
        return nil, dflhttp.New(http.StatusNotFound, "user_not_found", dflhttp.M{"id": req.ID})
    }
    return &user, nil
}

r := dflhttp.NewRouter(http.NewServeMux())
r.Handle(http.MethodGet, "/users/{id}", handleGet)
```

Errors go out as `{code, status_code, meta}`, plus `reasons` when you
attach per-field detail; every layer is swappable, from mapping your own
error types onto that shape to owning the response body outright.

More: [http guide](./http/README.md) ·
[runnable examples](./http/examples) ·
[custom error envelope](./http/examples/errorwriter)

## http/version

One route, a handler per API version. You choose where versions travel
(header, query, path, your own extractor) and how they match: Stripe-style
inheritance, where a pin gets the newest variant not newer than it, or
fully explicit, where every version is declared and `latest` is the one
moving pointer. A `preview` overlay and a status response header are both
opt-in.

```go
api := version.NewResolver(version.Dates(),
    version.Header("X-API-Version"),
).AllowLatest("latest")

users := version.NewEndpoint(api)
version.Handle(users, "2024-01-02", listUsersV1)
version.Handle(users, "2024-06-01", listUsersV2)

r.HandleFunc(http.MethodGet, "/users", users.Serve)
```

More: [version guide](./http/version/README.md) ·
[runnable examples](./http/version/example_test.go)

## events

The producer/consumer twin of `http`. Events name themselves, handlers
are `func(ctx, E) error`, and the same handler can subscribe in-process
(`bus.On`) or serve as `POST /events/{name}` (`bus.RegisterEndpoint`).
Swap the in-memory sink for SQS or Pub/Sub and nothing above it changes.

```go
type UserCreated struct {
    ID    string `json:"id"`
    Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

bus := events.NewBus(events.NewMemSink())
bus.On(func(ctx context.Context, e UserCreated) error {
    log.Printf("welcome %s", e.Email)
    return nil
})
_ = bus.Emit(ctx, UserCreated{ID: "1", Email: "a@b.com"})
```

Plugins wrap both sides of the trip; that's how [`events/otel`](./events/otel)
carries one trace from emitter to handler.

More: [events guide](./events/README.md) ·
[runnable examples](./events/examples) ·
[writing a sink](./events/README.md#writing-a-sink)

## db/pgxdb

Transaction shapes (`Tx`, `TxRead`, `TxSerializable` with retry), generic
`Get`/`Scalar`/`Select` scanners, and transactions carried on the
context: a repository written against `GetQuerier(ctx, pool)` joins the
ambient transaction when there is one and uses the pool when there isn't.

```go
db.TxCtx(ctx, func(ctx context.Context) error {
    if _, err := repo.Create(ctx, "Alice", "alice@example.com"); err != nil {
        return err
    }
    _, err := repo.Create(ctx, "Bob", "bob@example.com")
    return err // one transaction, or neither insert
})
```

More: [pgxdb guide](./db/pgxdb/README.md) ·
[runnable examples](./db/pgxdb/examples)

## Development

```
task test   # go test for every module
task lint   # golangci-lint + staticcheck for every module
```

CI runs the same, per [ci.yml](./.github/workflows/ci.yml). The router
and bus each ship a conformance suite
([`http/internal/routertest`](./http/internal/routertest),
[`events/internal/bustest`](./events/internal/bustest)) that runs the
full behaviour matrix against every backend, so a new mux or sink starts
from a passing spec.
