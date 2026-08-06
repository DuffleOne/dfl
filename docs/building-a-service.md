# Building a service with dfl

This guide wires all three libraries into one small service: an HTTP API
that creates and fetches users, persists them with `pgxdb`, and emits a
`user.created` event that a consumer picks up. Every fragment is real API;
the runnable per-package versions live in the `examples/` directories.

The finished shape:

```
cmd/api/main.go        wiring: db, bus, router
internal/store/        pgxdb repository
internal/api/          http handlers
internal/apievents/    event types and consumers
```

## The store

A repository method takes its real arguments only. No `pgx.Tx` parameter:
`GetQuerier` resolves to the ambient transaction when one is on the context
and to the pool otherwise.

```go
package store

type Users struct {
	db *pgxdb.DB
}

func NewUsers(db *pgxdb.DB) *Users { return &Users{db: db} }

func (s *Users) Create(ctx context.Context, name, email string) (User, error) {
	q := pgxdb.GetQuerier(ctx, s.db)

	return pgxdb.Get[User](ctx, q,
		`INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email`,
		name, email)
}

func (s *Users) Get(ctx context.Context, id int64) (User, error) {
	q := pgxdb.GetQuerier(ctx, s.db)

	return pgxdb.Get[User](ctx, q,
		`SELECT id, name, email FROM users WHERE id = $1`, id)
}
```

`pgxdb.Get` returns `pgxdb.NotFound` for zero rows; the handler layer will
turn that into a 404. When one request needs several store calls to be
atomic, the caller wraps them in `db.TxCtx` and the methods join the
transaction without changing shape.

## The events

An event is a struct that names itself, and optionally validates itself.
Validation runs on both publish and delivery, so a producer can't emit a
broken event and a consumer can't act on one that arrived broken over a real
transport.

```go
package apievents

type UserCreated struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func (e UserCreated) Validate() error {
	if e.Email == "" {
		return events.New("validation_failed", events.M{"fields": events.M{"email": "is required"}})
	}

	return nil
}
```

Consumers are plain functions, registered on the bus at boot:

```go
func Subscribe(bus *events.Bus, mailer *Mailer) {
	bus.On(func(ctx context.Context, e UserCreated) error {
		return mailer.SendWelcome(ctx, e.Email)
	})
}
```

`On` delivery is asynchronous. A consumer error can't reach whoever called
`Emit`, so it goes to the bus `ErrorHandler` (and back to the sink, which is
how a durable transport knows to redeliver). The default handler logs via
`slog`; point it at your alerting with `events.WithErrorHandler`.

## The handlers

Handlers speak types on the way in and out, and errors do double duty:
HTTP-shaped for the client, `errors.Is`-traversable for logs and callers.

```go
package api

type Users struct {
	store *store.Users
	bus   *events.Bus
}

type CreateUserReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (u *Users) handleCreate(ctx context.Context, req *CreateUserReq) (*store.User, error) {
	if fields := req.validate(); fields != nil {
		return nil, dflhttp.New("validation_failed", dflhttp.M{"fields": fields})
	}

	user, err := u.store.Create(ctx, req.Name, req.Email)
	if err != nil {
		return nil, err // the coercer picks it up below
	}

	// Emit after the write is committed. See "Emitting and transactions".
	if err := u.bus.Emit(ctx, apievents.UserCreated{ID: user.ID, Email: user.Email}); err != nil {
		return nil, err
	}

	return &user, nil
}

type GetUserReq struct {
	ID int64 `path:"id"`
}

func (u *Users) handleGet(ctx context.Context, req *GetUserReq) (*store.User, error) {
	user, err := u.store.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (u *Users) Mount(rg *dflhttp.Router) {
	rg.Handle(http.MethodPost, "/users", u.handleCreate)
	rg.Handle(http.MethodGet, "/users/{id}", u.handleGet)
}
```

`handleGet` doesn't mention 404 at all. That mapping belongs in one place, a
`Coercer` installed on the router, rather than at every call site:

```go
func coerce(err error) *dflhttp.ReqError {
	if errors.Is(err, pgxdb.NotFound) {
		return dflhttp.Wrap(err, "not_found", nil)
	}

	return dflhttp.DefaultCoercer(err)
}
```

If your codebase already has an error envelope of its own and dfl's
`{code, meta, reasons}` shape isn't wanted on the wire, skip the coercer
and set `dflhttp.WithErrorWriter` instead; the
[http guide](../http/README.md#errors) draws the line between the two.

## The wiring

```go
package main

func main() {
	ctx := context.Background()

	db, err := pgxdb.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer db.Close()

	bus := events.NewBus(events.NewMemSink(), events.WithPlugins(otel.New()))
	apievents.Subscribe(bus, mailer)

	r := dflhttp.NewRouter(http.NewServeMux(), dflhttp.WithCoercer(coerce))
	r.Use(requestLogging)

	api.NewUsers(store.NewUsers(db), bus).Mount(r.Group("/api"))

	log.Fatal(http.ListenAndServe(":8080", r))
}
```

Registration order matters in one place only: install plugins and call
`bus.Use` before registering handlers, since both apply to handlers
registered afterwards. The same goes for `r.Use` and routes.

## Emitting and transactions

`handleCreate` emits after `store.Create` returns, which is after the
insert committed. Resist the pull to emit from inside a `TxCtx` closure: the
publish would happen before the commit, so a rollback would leave you having
announced a user that doesn't exist. The reverse failure mode still exists
(commit succeeds, emit fails, client gets an error for work that happened),
which is inherent to two systems without a shared log.

When that window matters, the fix is an outbox: insert the envelope into an
outbox table inside the transaction, and run a relay that reads the table
and publishes through the bus. The `Sink` interface is deliberately small so
an outbox relay can stand behind it; the bus doesn't care that publishes are
deferred.

## Testing

Handlers are functions, so most tests never touch HTTP:

```go
_, err := u.handleCreate(ctx, &CreateUserReq{Name: "x", Email: ""})
// assert the ReqError's code is validation_failed
```

Routing-level behaviour runs through `httptest` against the router, no
listener involved. Async consumers are awaited with a `sync.WaitGroup`, the
pattern `events/internal/bustest` uses throughout: `Add` before emitting,
`Done` in the consumer (or the error handler), `Wait` before asserting.
`events/examples/app/app_test.go` shows both styles against a real bus.

## Growing up

The parts are designed to be swapped without the application changing:

- MemSink to a real transport: construct an SQS or Pub/Sub sink from
  [`events/aws`](../events/aws) or [`events/gcp`](../events/gcp) instead.
  Handlers, events, and emit sites stay identical; a worker binary calls
  `sink.Receive(ctx)`, or a push subscription POSTs into the mounted sink.
- Consumers as HTTP endpoints: `bus.RegisterEndpoint(rg, handler)` serves
  any consumer at `POST /events/{name}`, useful when the transport pushes
  events over HTTP or another service wants to hand you one synchronously.
- `*http.ServeMux` to chi (or back): change the argument to `NewRouter`.
- Plain queries to transactions: wrap call sites in `db.TxCtx`; repositories
  built on `GetQuerier` follow along.
