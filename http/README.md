# http

Typed HTTP handlers on top of `net/http`, with structured errors and a
pluggable mux. Import it aliased, so it doesn't clash with the stdlib at use
sites:

```go
import dflhttp "github.com/duffleone/dfl/http"
```

Needs Go 1.27 or later: the router uses generic methods, added in that
release.

## The handler model

A handler is a function, not an interface:

```go
func(ctx context.Context, req *Req) (*Resp, error)
```

The router binds `Req` from the request (path, query, JSON body), calls the
handler, then JSON-encodes `Resp` on success or writes an error response on
failure. There is no `http.ResponseWriter` in sight, so a handler is trivially
callable from a test, another handler, or a queue consumer.

Convention: `Req` and `Resp` are pointer-to-struct. The error path is then
just `return nil, err`, and `*dflhttp.Empty` covers routes with no input or
output of substance (an `Empty` response produces `204 No Content`). Value
shapes also work, but the pointer form is what every example uses.

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
        return nil, dflhttp.New("not_found", dflhttp.M{"resource": "user", "id": req.ID})
    }

    return &user, nil
}

func main() {
    r := dflhttp.NewRouter(http.NewServeMux())
    r.Handle(http.MethodGet, "/users/{id}", handleGet)
    log.Fatal(http.ListenAndServe(":8080", r))
}
```

## Request binding

Fields of `Req` bind by struct tag:

| Tag            | Source                              | On failure                    |
| -------------- | ----------------------------------- | ----------------------------- |
| `path:"name"`  | `r.PathValue("name")`               | 400 `invalid_path_param`      |
| `query:"name"` | `r.URL.Query().Get("name")`         | 400 `invalid_query_param`     |
| `json:"name"`  | that key of the JSON request body   | 400 `invalid_body_field`      |

Path and query values are strings on the wire, parsed into the field's type:
strings, bools, ints, uints, floats, and anything implementing
`encoding.TextUnmarshaler` work out of the box. A missing path or query value
leaves the field at its zero value; requiredness is the handler's job (see
the validation pattern below).

Body binding touches only `json`-tagged fields, each decoded individually so
the error can name the field. A key in the body that happens to match a
path-bound field's name cannot overwrite it: path and query fields are simply
not part of the body's field set. A request with a non-JSON `Content-Type`
and a body-shaped `Req` fails fast with 415 `unsupported_media_type`;
malformed JSON is a 400 `invalid_body`.

The binding plan for each `Req` type is built by reflection once, at
registration, and cached. A `Req` the parser can't bind (say a `[]string`
path field with no setter for it) panics at `Handle` time, not on the first
request.

### Custom binding

`RequestParser` has two hooks, set on a parser you pass via
`WithRequestParser`:

`Setters` teaches path and query binding new field types (or overrides how
known ones parse):

```go
parser := &dflhttp.RequestParser{
    Setters: map[reflect.Type]dflhttp.SetterFunc{
        reflect.TypeFor[[]string](): func(dst reflect.Value, raw string) error {
            dst.Set(reflect.ValueOf(strings.Split(raw, ",")))

            return nil
        },
    },
}

r := dflhttp.NewRouter(http.NewServeMux(), dflhttp.WithRequestParser(parser))
```

`DecodeBody` replaces JSON body binding wholesale, for a different wire
format. Path and query fields are already bound by the time it runs.

## Muxes

`NewRouter` wraps any mux that can register handlers in one of two shapes:

- `MethodFunc(method, pattern string, h http.HandlerFunc)`, the chi style
  (`MethodMux`), or
- `HandleFunc("METHOD /pattern", h)`, the stdlib style on Go 1.22+
  (`PatternMux`).

Both `*http.ServeMux` and `*chi.Mux` satisfy one of these directly, and the
package has no awareness of either beyond that. The conformance suite in
`internal/routertest` runs the full behaviour matrix against both backends,
so switching mux is a one-line change.

## Groups and middleware

```go
r := dflhttp.NewRouter(http.NewServeMux())
r.Use(requestLogging)

api := r.Group("/api")
api.Use(auth)

api.Handle(http.MethodGet, "/users/{id}", handleGet)          // logged + authed
api.Handle(http.MethodPost, "/users", handleCreate, rateLimit) // plus per-route mw
```

Middleware operates one level below typed handlers:

```go
type Middleware func(next HandlerFunc) HandlerFunc

type HandlerFunc func(w http.ResponseWriter, r *http.Request) error
```

It composes in onion order (first `Use`d sees the request first), can
short-circuit by returning an error without calling `next`, and can transform
the error `next` returned. Because middleware returns an error rather than
writing a response, an auth failure goes through exactly the same error
pipeline as a handler failure.

`Group` inherits the parent's configuration (coercer, error writer, request
parser) and a snapshot of its middleware; `Use` on the parent after `Group`
does not reach the child. Routes the typed model doesn't fit (SSE, websocket
upgrades, custom content types) register raw with `r.HandleFunc`.

## Errors

The error pipeline has three layers. Most services only ever touch the first.

### 1. `ReqError`, the built-in shape

```go
return nil, dflhttp.New("not_found", dflhttp.M{"resource": "user", "id": req.ID})
```

On the wire it serialises as:

```json
{"code": "not_found", "meta": {"resource": "user", "id": "42"}}
```

There's no status in the body. It's on the status line already, and leaving
it off makes the shape identical to
[cher](https://github.com/wearemojo/mojo-public-go/tree/main/lib/cher)'s, so
a service can move between the two without its clients noticing.

The causes attached with `New`'s variadic tail or `Wrap` never serialise;
they exist for `errors.Is` and `errors.As` traversal, so a handler can both
speak HTTP and preserve the underlying cause for callers and logs.

Detail the caller should see goes the other way, as `Reasons`: structured,
machine-readable entries that do serialise, one per failed check, so a
client learns everything wrong with its request in one round trip rather
than fixing one field per attempt:

```go
return nil, dflhttp.New("invalid_team", nil).
    WithReasons(
        dflhttp.Reason{Code: "required", Meta: dflhttp.M{"in": "body", "field": "name"}},
        dflhttp.Reason{Code: "invalid", Meta: dflhttp.M{"in": "body", "field": "size", "message": "must be 1 or greater"}},
    )
```

```json
{"code": "invalid_team",
 "reasons": [{"code": "required", "meta": {"field": "name", "in": "body"}},
             {"code": "invalid", "meta": {"field": "size", "message": "must be 1 or greater", "in": "body"}}]}
```

Reasons nest, so a check that decomposes keeps its shape rather than
flattening into one list where the client has to work out which failure
belongs to which field:

```go
dflhttp.Reason{Code: "invalid", Meta: dflhttp.M{"in": "body", "field": "address"},
    Reasons: []dflhttp.Reason{
        {Code: "required", Meta: dflhttp.M{"field": "postcode"}},
    }}
```

`WithReasons` copies rather than mutates, so a package-level sentinel
`ReqError` can be decorated per request safely. Causes are for your logs;
reasons are for the caller.

#### The status comes from the code

`StatusCode()` derives the HTTP status from `Code`, and **the default is
400**. A `ReqError` is an error somebody wrote down on purpose, which makes
it part of the contract rather than something that should page anyone. The
codes that genuinely are the server's fault say so.

| Code                                    | Status |
| --------------------------------------- | ------ |
| `bad_request`, and anything unlisted    | 400    |
| `unauthorized`                          | 401    |
| `access_denied`                         | 403    |
| `not_found`, `route_not_found`          | 404    |
| `method_not_allowed`                    | 405    |
| `endpoint_withdrawn`                    | 410    |
| `unsupported_media_type`                | 415    |
| `too_many_requests`                     | 429    |
| `unknown`                               | 500    |

That's cher's table, plus `unsupported_media_type` for the one the request
parser produces. It's deliberately small: the code is the contract your
client matches on, and the status is the coarse bucket in front of it. Push
the specifics into the code and the meta, not into a status nobody can
branch on.

For the statuses outside it (a 409 on a conflicting write, a 402, a 502 from
a dead upstream) say so directly:

```go
return nil, dflhttp.New("team_name_taken", dflhttp.M{"name": req.Name}).
    WithStatus(http.StatusConflict)
```

`WithStatus` changes the status line only, and copies like `WithReasons`
does. The body is untouched, so the client still matches on `code`.

### 2. `Coercer`, mapping your errors onto `ReqError`

A `Coercer` is `func(error) *ReqError`. The default passes `*ReqError`
through and turns anything else into `unknown`, which is one of the codes
that does mean 500: an error nothing classified is a bug, not a contract.
Plug your own in when handlers return domain errors and you want the mapping
in one place:

```go
r := dflhttp.NewRouter(mux, dflhttp.WithCoercer(func(err error) *dflhttp.ReqError {
    if errors.Is(err, pgxdb.NotFound) {
        return dflhttp.Wrap(err, "not_found", nil)
    }

    return dflhttp.DefaultCoercer(err)
}))
```

Two ready-made coercers ship with the package: [`http/oops`](./oops) for
`samber/oops` errors, and [`http/cher`](./cher) for
[`cher`](https://github.com/wearemojo/mojo-public-go/tree/main/lib/cher),
which takes its status from cher's own code-to-status table.

### 3. `ErrorWriter`, owning the response outright

A `Coercer` still ends at `ReqError`'s wire shape. When that shape itself is
the problem, because your codebase already has an error envelope it wants to
emit verbatim, set an `ErrorWriter` instead:

```go
r := dflhttp.NewRouter(mux, dflhttp.WithErrorWriter(
    func(w http.ResponseWriter, r *http.Request, err error) {
        var apiErr *myapp.Error
        if errors.As(err, &apiErr) {
            writeMyShape(w, apiErr) // your status, your body, reasons and all

            return
        }

        fallback(w, r, err)
    }))
```

The writer receives the error exactly as the handler, middleware, or binding
returned it, before any coercion, and the `Coercer` is never consulted.
Binding failures arrive as the `*ReqError` values the parser produced, so a
writer that owns the wire shape should map those too; a writer that's happy
to emit dfl's shape for anything it doesn't recognise can keep
`dflhttp.DefaultErrorWriter(nil)` around as its `fallback`.

Rule of thumb: reach for `WithCoercer` to classify errors into dfl's shape,
and `WithErrorWriter` to escape the shape entirely.
[`examples/errorwriter`](./examples/errorwriter) is a runnable service doing
the latter.

## Validation in one round trip

Binding gets types right; requiredness and domain rules belong to the
handler. The pattern used across the examples collects every failure before
returning, so the client fixes everything at once:

```go
func (r *CreateWidgetReq) validate() dflhttp.M {
    fields := dflhttp.M{}

    if r.Qty < 1 || r.Qty > 100 {
        fields["qty"] = "must be between 1 and 100"
    }

    if strings.TrimSpace(r.Name) == "" {
        fields["name"] = "is required"
    }

    if len(fields) == 0 {
        return nil
    }

    return fields
}

func handleCreate(_ context.Context, req *CreateWidgetReq) (*Widget, error) {
    if fields := req.validate(); fields != nil {
        return nil, dflhttp.New("validation_failed", dflhttp.M{"fields": fields})
    }
    // ...
}
```

[`examples/api/widgets.go`](./examples/api/widgets.go) shows the full version
validating path, query, and body together.

## Versioning

Endpoints that need to change shape without breaking pinned clients get a
handler variant per version through [`http/version`](./version): a
`Scheme` says what versions are (dates, `v1`/`v2`, anything you can parse
and compare), `Source`s say where the version travels on the request (a
header, a query parameter, your own function), and each request runs the
newest variant not newer than its pin.

```go
api := version.NewResolver(version.Dates(), version.Header("X-API-Version"))

users := version.NewEndpoint(api)
version.Handle(users, "2024-01-02", listUsersV1)
version.Handle(users, "2024-06-01", listUsersV2)

r.HandleFunc(http.MethodGet, "/users", users.Serve)
```

It's built on `Adapt`, the exported half of `Handle`'s typed plumbing:
`Adapt` turns a typed handler into a `HandlerFunc` without registering it,
for any dispatch decision that isn't a route. The
[version guide](./version/README.md) covers schemes, sources, matching
rules, and errors.

## Testing handlers

Typed handlers are plain functions: call them. For routing-level behaviour,
drive the router with `httptest`, no server needed:

```go
r := dflhttp.NewRouter(http.NewServeMux())
r.Handle(http.MethodGet, "/users/{id}", handleGet)

req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
rec := httptest.NewRecorder()
r.ServeHTTP(rec, req)
```

## Examples

- [`examples/std`](./examples/std): full API on stdlib `*http.ServeMux`
- [`examples/chi`](./examples/chi): the same API on `*chi.Mux`, with the oops coercer
- [`examples/errorwriter`](./examples/errorwriter): a service that owns its error wire shape via `WithErrorWriter`
- [`examples/api`](./examples/api): the shared handlers, including multi-source validation
