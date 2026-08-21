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
	if reasons := req.validate(); len(reasons) > 0 {
		return nil, dflhttp.New("validation_failed", nil).WithReasons(reasons...)
	}

	// Emit before the write, and mint the id here so the event can name the
	// user before the row exists. Consumers that arrive early retry until it
	// does. See "Emitting and transactions".
	user := store.User{ID: newID(), Name: req.Name, Email: req.Email}

	if err := u.bus.Emit(ctx, apievents.UserCreated{ID: user.ID, Email: user.Email}); err != nil {
		return nil, err
	}

	if err := u.store.Create(ctx, user); err != nil {
		return nil, err // the coercer picks it up below
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

`validate` returns `[]dflhttp.Reason` entries shaped `{in, field}`, the
same contract the binder uses for its own failures, so the client parses
one shape for "didn't bind" and "didn't pass the rules"; the
[http guide](../http/README.md#validation-in-one-round-trip) has the full
pattern.

`handleGet` doesn't mention 404 at all, and `handleCreate` doesn't mention
the duplicate-email case. Those mappings belong in one place, a `Coercer`
installed on the router, rather than at every call site:

```go
func coerce(err error) *dflhttp.ReqError {
	if errors.Is(err, pgxdb.NotFound) {
		return dflhttp.Wrap(err, "not_found", nil)
	}

	if pgxdb.IsUniqueViolation(err, "users_email_key") {
		return dflhttp.Wrap(err, "email_taken", nil).WithStatus(http.StatusConflict)
	}

	return dflhttp.DefaultCoercer(err)
}
```

The second branch is pgxdb's constraint classification doing the work: the
store never inspects the error, the unique index on `email` speaks through
`IsUniqueViolation`, and naming the index means a second unique constraint
on the table won't silently start reporting `email_taken` too.

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

	mux := http.NewServeMux()
	mux.HandleFunc("/", dflhttp.NotFoundHandler()) // unmatched routes get dfl's shape

	r := dflhttp.NewRouter(mux, dflhttp.WithCoercer(coerce))
	r.Use(dflhttp.Recoverer()) // a panicking handler 500s instead of dropping the connection
	r.Use(requestLogging)

	r.Handle(http.MethodGet, "/health", func(ctx context.Context, _ *dflhttp.Empty) (*dflhttp.Empty, error) {
		return nil, db.Ping(ctx) // prove the database, not just the listener
	})

	api.NewUsers(store.NewUsers(db), bus).Mount(r.Group("/api"))

	log.Fatal(http.ListenAndServe(":8080", r))
}
```

The health route goes through `db.Ping` because a check that only proves
the HTTP listener accepts connections reports healthy while pointing at a
database that no longer exists; a pool that can't reach postgres is a
service that can't serve requests.

Registration order matters in one place only: install plugins and call
`bus.Use` before registering handlers, since both apply to handlers
registered afterwards. The same goes for `r.Use` and routes.

## Emitting and transactions

Two systems, no shared log, so something has to give. Order the write and the
emit either way and one of them can happen without the other.

`handleCreate` emits first, before the transaction that writes the user. That
sounds backwards, and the reason it works is on the consuming side: a
consumer that looks for a user and doesn't find one hasn't failed, it's early.
So it waits and looks again.

```go
bus.Use(events.Retry(events.RetryPolicy{}))
```

```go
bus.On(func(ctx context.Context, e apievents.UserCreated) error {
	user, err := store.Get(ctx, e.ID)
	if errors.Is(err, pgxdb.NotFound) {
		return events.NotFound // retried, with backoff
	}
	// ...
})
```

Three attempts by default, backing off 50ms then 100ms, and only on
not-found: a genuine bug still fails once rather than three times. See
[the retry guide](../events/README.md#arriving-early) for the knobs.

Emitting first means minting the id yourself rather than letting the database
do it, since the event has to name the user before the row exists:

```go
user := store.User{ID: newID(), Name: req.Name, Email: req.Email}

if err := u.bus.Emit(ctx, apievents.UserCreated{ID: user.ID, Email: user.Email}); err != nil {
	return nil, err
}

if err := u.store.Create(ctx, user); err != nil {
	return nil, err
}
```

### When the retries run out

If the user still isn't there after the last attempt, the write didn't just
lag, it didn't happen: the transaction rolled back, or the process died
between the emit and the commit. The event describes something that doesn't
exist and never will, and no amount of further waiting fixes it. Choose what
happens to it:

| `Exhausted`                   | What it does                                          |
| ----------------------------- | ----------------------------------------------------- |
| `events.Nack` (default)       | Logs, hands the error to the sink, and lets the transport's own redrive policy decide |
| `events.DeadLetter(park)`     | Logs, hands the envelope to `park`, and acks           |
| `events.Drop`                 | Logs and acks. The event is gone                       |

```go
bus.Use(events.Retry(events.RetryPolicy{
	Attempts:  3,
	Exhausted: events.DeadLetter(parkForReview),
}))
```

`Drop` is right for an advisory event (invalidate a cache, refresh a
projection) where a lost one costs nothing. `DeadLetter` is right when
somebody needs to look at it, which is most of the time. `Nack` is right when
you're on SQS or Pub/Sub and their dead-letter machinery already has an
answer you don't want to overrule.

### The real fix is an outbox

Retries and a dead-letter queue make the failure survivable, not impossible.
The event still went out for work that never happened, and everything
downstream that reacted to it, an email, a charge, a projection, has to be
unwound by hand.

The fix that removes the failure rather than absorbing it is a transactional
outbox: write the event into a table on the same transaction as the business
data, and have a relay move rows from that table onto the real transport
after the commit. A rollback takes the event with it, and a commit guarantees
it goes out. No phantom events, no lost ones, and no need to mint ids early.

```go
outbox := newOutboxSink(db, sqsSink)   // Publish writes a row; Subscribe forwards
bus := events.NewBus(outbox)

go outbox.Relay(ctx, 100*time.Millisecond, 50)

db.TxCtx(ctx, func(ctx context.Context) error {
	id, err := store.Create(ctx, req.Name, req.Email)
	if err != nil {
		return err
	}

	// Lands in the outbox table on this same transaction.
	return bus.Emit(ctx, apievents.UserCreated{ID: id, Email: req.Email})
})
```

`Sink` is deliberately small so this fits behind it, and `pgxdb.GetQuerier`
is what makes it work: called inside a `TxCtx` block it hands back the
running transaction, so the insert joins it without the sink knowing whether
there was one.

What it costs you: a table and a relay to run, and at-least-once delivery
rather than exactly-once, since a publish that succeeds before the marking
commit fails will go out twice. Consumers have to tolerate duplicates. That's
a much easier property to design for than phantom events.

[`events/examples/outbox`](../events/examples/outbox) is the whole thing
working: the sink, the relay, a committed write whose event arrives, and a
rolled back one whose event never does.

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
