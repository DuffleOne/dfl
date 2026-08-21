# db/pgxdb

A thin wrapper around `jackc/pgx/v5`: transaction shapes, generic row
scanning, and a context-carried transaction so repository code doesn't
thread `pgx.Tx` through every signature.

```go
import "github.com/duffleone/dfl/db/pgxdb"

db, err := pgxdb.New(ctx, os.Getenv("DATABASE_URL")) // pools + pings
defer db.Close()
```

`New` takes options that adjust the parsed `*pgxpool.Config` before the
pool is built, so pool sizing and tracing don't force you off the
constructor:

```go
db, err := pgxdb.New(ctx, url, func(cfg *pgxpool.Config) {
    cfg.MaxConns = 20
})
```

The default query exec mode is `DescribeExec`, not pgx's statement cache.
The cache holds each statement's plan per connection, so a migration that
changes a result shape leaves every pooled connection failing with `cached
plan must not change result type` (SQLSTATE `0A000`) until it cycles: a
burst of intermittent 500s after any deploy that touches a column. Plain
`Exec` mode avoids that but guesses parameter types, which breaks `json`
and enum parameters. `DescribeExec` describes on every execution and caches
nothing, the only mode that fixes the first problem without causing the
second. An option can put the cache back if you never migrate against a
live pool.

`db.Ping(ctx)` verifies the pool can actually reach postgres. Wire it into
your health check: a service whose check only proves the HTTP listener
accepts connections will happily report healthy while pointing at a
database that no longer exists.

## Querier

```go
type Querier interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

Both `*pgxdb.DB` (the pool) and `pgx.Tx` satisfy it, so every helper in the
package, and every repository function you write against it, works the same
inside or outside a transaction. This one interface is what the rest of the
package hangs off.

## Reading rows

Three generic helpers cover the usual shapes:

```go
user, err := pgxdb.Get[User](ctx, db,      // exactly one struct row
    `SELECT id, name, email FROM users WHERE id = $1`, id)

n, err := pgxdb.Scalar[int](ctx, db,       // exactly one value
    `SELECT count(*) FROM users`)

all, err := pgxdb.Select[User](ctx, db,    // zero or more struct rows
    `SELECT id, name, email FROM users ORDER BY id`)
```

`Get` and `Select` match columns to struct fields by `db` tag, or lowercased
field name when untagged. `Scalar` takes anything pgx can scan: ints,
strings, `time.Time`, `uuid.UUID`, `sql.Scanner` implementations. `Get` and
`Scalar` return `pgxdb.NotFound` on zero rows (an alias of `pgx.ErrNoRows`,
so `errors.Is` matches either name) and an error on more than one.
`Select` returns an empty slice, not an error, for no rows.

`RETURNING` makes the write helpers unnecessary:

```go
user, err := pgxdb.Get[User](ctx, db,
    `INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email`,
    name, email)
```

### Strict and lax scanning

`Get` and `Select` are strict: a struct field with no matching column in
the result set is an error. That catches a mistyped `db` tag, which would
otherwise scan as a silent zero value. `GetLax` and `SelectLax` relax it:
unmatched fields keep their zero value.

Lax is for partial selects over a wide shared model, the unified-struct
style where one type carries both `db` and `json` tags and serves the
repository and the API at once. Such a model routinely has fields only some
queries populate (derived counts, joined display names, columns a summary
query skips for cost), and under strict scanning every one of those queries
fails. The tradeoff runs both ways, which is why both exist: strict catches
tag typos, lax supports partial selects, and a codebase can reasonably want
each in different places.

## Constraint errors

The read helpers classify driver errors on the way out: a constraint-class
SQLSTATE gets a package sentinel wrapped onto the chain, so repositories
match with `errors.Is` instead of importing `pgconn` and comparing codes.

```go
_, err := pgxdb.Get[User](ctx, db,
    `INSERT INTO users (email) VALUES ($1) RETURNING id, email`, email)

switch {
case pgxdb.IsUniqueViolation(err, "users_email_key"):
    return ErrEmailTaken
case errors.Is(err, pgxdb.ErrForeignKeyViolation):
    return ErrNoSuchTeam
}
```

The sentinels: `ErrUniqueViolation`, `ErrForeignKeyViolation`,
`ErrCheckViolation`, `ErrNotNullViolation`, `ErrExclusionViolation`, and
`ErrSerializationFailure` (which also covers deadlock, `40P01`). The
underlying `*pgconn.PgError` stays on the chain for `errors.As`, and
`pgxdb.ConstraintName(err)` pulls the violated constraint's name out
directly. Anything unrecognised, `NotFound` included, passes through
untouched.

`IsUniqueViolation(err, constraint)` matches the code and, when constraint
is non-empty, the specific index. That second argument is the useful part:
a table with several unique indexes raises the same SQLSTATE for each, so
"was this a unique violation" can't tell the email collision from the
external-id one, and mapping every collision to one domain error is wrong
the moment a second index exists. `IsExclusionViolation` is its sibling for
`EXCLUDE` constraints, which raise `23P01` rather than `23505` and are
invisible to unique-violation checks; a `tstzrange` overlap exclusion is
the classic case. Both work on raw driver errors too, so they don't care
whether the error came through a helper.

Deliberately, none of this is HTTP-aware. What a unique violation means
depends entirely on which write hit it, so translating one to a status code
is the caller's job; left alone they stay 500s, which is the right answer
for a constraint nobody thought about.

## Transactions

Three shapes, each a closure that commits on nil and rolls back on error or
panic:

```go
db.Tx(ctx, f)             // read committed
db.TxRead(ctx, f)         // repeatable read, read-only
db.TxSerializable(ctx, f) // serializable, retried on conflict
```

`TxSerializable` retries the closure up to three times when postgres
reports a serialization failure (`40001`) or a deadlock (`40P01`), both
equally transient and equally safe to re-run as a fresh transaction,
logging a warning between attempts. The closure must therefore be safe to
re-run: pure reads and writes are fine, but don't send emails or publish
events from inside one. Any other error, or a cancelled context, returns
immediately. Conflicts surfacing at `COMMIT` rather than inside the
closure retry the same way.

## The context-carried transaction

The `f func(tx pgx.Tx) error` shape forces every function called inside the
transaction to accept the `tx`. The `Ctx` variants attach the transaction to
the context instead:

```go
db.TxCtx(ctx, func(ctx context.Context) error {
    if _, err := repo.Create(ctx, "Alice", "alice@example.com"); err != nil {
        return err
    }

    _, err := repo.Create(ctx, "Bob", "bob@example.com")

    return err // both inserts in one transaction, or neither
})
```

Repository functions then resolve their `Querier` with `GetQuerier`, which
returns the running transaction when there is one and the fallback
otherwise:

```go
func (r *UsersRepo) Create(ctx context.Context, name, email string) (User, error) {
    q := pgxdb.GetQuerier(ctx, r.db)

    return pgxdb.Get[User](ctx, q,
        `INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email`,
        name, email)
}
```

The same method now runs against the pool when called normally and joins the
ambient transaction when called inside a `TxCtx` block, with no change at
the call site. `TxReadCtx` and `TxSerializableCtx` complete the family (each
serializable retry gets a fresh transaction on a fresh derived context), and
`TxFromContext` exposes the raw `pgx.Tx` for the rare case that needs it.

## Escape hatch

`db.Std()` returns a `*sql.DB` sharing the same pool, plus a cleanup
function, for libraries that only speak `database/sql` (migration runners,
mostly).

## With the http package

`NotFound` is the natural partner of a 404 mapping, either per call site or
once in a `Coercer`:

```go
user, err := users.Get(ctx, db, req.ID)
if errors.Is(err, pgxdb.NotFound) {
    return nil, dflhttp.New("not_found", dflhttp.M{"resource": "user", "id": req.ID})
}
```

## Examples

Each needs a `DATABASE_URL`:

- [`examples/basic`](./examples/basic): the helpers against the pool
- [`examples/tx`](./examples/tx): a repository joining an ambient `TxCtx` transaction
- [`examples/serializable`](./examples/serializable): serializable retries under contention
- [`examples/users`](./examples/users): the shared repository they all use
