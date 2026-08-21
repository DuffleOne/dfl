# version

Per-endpoint API versioning for `dflhttp`. One route, several handler
variants, each pinned to the version that introduced it; the package
resolves the version a request asks for and picks the variant before the
underlying function runs.

```go
import "github.com/duffleone/dfl/http/version"
```

Part of the root module, no extra dependency. Nothing here touches the
server side of `dflhttp`: the package plugs in at the handler level, so it
works with any mux the Router wraps.

## The model

Three pluggable pieces:

- A `Scheme` gives versions their type and order: dates, `v1`/`v2`
  counters, or anything you can parse and compare.
- `Source`s say where the version travels on the request: a header, a
  query parameter, a path segment, or a function of your own.
- An `Endpoint` holds one handler per version and dispatches.

A `Resolver` pairs a scheme with sources once per API, and every endpoint
shares it:

```go
api := version.NewResolver(version.Dates(),
    version.Header("X-API-Version"),
    version.Query("api_version"),
)

users := version.NewEndpoint(api)
users.Handle("2024-01-02", listUsersV1)
users.Handle("2024-06-01", listUsersV2)

r := dflhttp.NewRouter(http.NewServeMux())
r.HandleFunc(http.MethodGet, "/users", users.Serve)
```

Variants are the same typed handlers the Router takes,
`func(ctx, *Req) (*Resp, error)`, and each version is free to have its own
`Req` and `Resp` shapes; that's rather the point. `Serve` is a
`dflhttp.HandlerFunc`, so a versioned endpoint registers as a raw route
and its failures flow through the Router's usual error pipeline.

Registration is fail-fast, like the Router's: a version the scheme can't
parse, a duplicate version, or a `Req` the parser can't bind panics at
startup, not on the first request. Register everything before serving
traffic; an `Endpoint` is not safe for concurrent registration.

## Schemes

Built-ins:

```go
version.Dates()              // "2024-06-01", the Stripe model
version.Sequential()         // "v1", "v2"; bare "2" also accepted
version.Ordered(strconv.Atoi) // any cmp.Ordered type + a parse func
```

`Dates` parses `YYYY-MM-DD` and orders chronologically: every breaking
change gets the date it shipped and clients pin the date they integrated
against. `Sequential` is the classic counter; the `v` prefix is optional
and case-insensitive on the wire. `Ordered` builds a scheme from just a
parse function for versions that are naturally an int, float, or string,
ordering with `cmp.Compare`, which suits things like app build numbers.

Anything else implements the interface directly:

```go
type Scheme[V any] interface {
    Parse(raw string) (V, error)
    Compare(a, b V) int
    Format(v V) string
}
```

Semver is the usual customer, since `"v10"` must sort above `"v9"` and
lexical order won't do: parse into a small struct, compare
component-wise, and hand the scheme to `NewResolver` like any built-in.

## Sources

A source is `func(r *http.Request) (raw string, ok bool)`. Built-ins:

```go
version.Header("X-API-Version")  // request header
version.Query("api_version")     // query parameter
version.PathValue("v")           // {v} bound by the router's pattern
version.Default("2024-06-01")    // always hits; place it last
```

The resolver tries sources in order and the first one to yield a value
decides the request. Two consequences worth knowing:

- A trailing `Default` catches requests that carry no version at all.
  Without one, a versionless request is a 400 `version_missing`.
- A value that fails to parse is a 400 `version_invalid` even when a
  `Default` sits behind it. A garbled explicit version should be loud,
  not silently swapped for a fallback.

Values are trimmed of surrounding whitespace; a whitespace-only value
counts as a miss.

Anything the built-ins don't cover is a function. An app build number
parsed out of `User-Agent`, with a header for other callers:

```go
appBuild := func(r *http.Request) (string, bool) {
    return buildFromUserAgent(r.Header.Get("User-Agent")) // "myapp/342 (ios)" -> "342"
}

api := version.NewResolver(version.Ordered(strconv.Atoi),
    appBuild,
    version.Header("X-App-Build"),
)
```

The same shape covers claims from an already-verified token, per-client
pins looked up from the request, and whatever else the deployment needs.

## Matching

The default rule is `MatchCompatible`: a request is served by the newest
variant that is not newer than its pin. A client pinned to `2024-03-15`
hits the `2024-01-02` variant above, and keeps hitting it as newer
variants land. That keeps endpoints sparse: when a breaking change ships,
only the endpoints whose behaviour changed register a new variant, and
every other endpoint carries on with the variants it already has.

A pin older than every variant is refused as `version_unsupported`, since
the endpoint has no idea how it behaved before it existed.

`MatchExact` is for the stricter regime where a pin must name a variant
outright:

```go
users := version.NewEndpoint(api, version.WithMatch(version.MatchExact))
```

Nothing carries between versions under `MatchExact`: an endpoint is
declared at every version it exists at, and a pin between declarations is
refused rather than served by a neighbour.

## The latest pseudo-version

Opt-in, and off by default: `AllowLatest` names request literals that mean
"the newest variant of whichever endpoint answers".

```go
api := version.NewResolver(version.Dates(),
    version.Header("X-API-Version"),
).AllowLatest("latest")
```

A request pinned `latest` is served by the endpoint's newest registered
variant, whatever the `Match` rule says, and `Resolved.Latest` on the
context records that it asked. Add a trailing `Default("latest")` source
to give requests carrying no version at all the same meaning, instead of
the 400 they'd otherwise get.

Literals are matched exactly after the usual whitespace trim and are
checked before the scheme parses; one the scheme could itself parse
panics at wiring, since it would shadow a real version. Enabling several
(`AllowLatest("latest", "null")`, for clients that serialise an unset pin
as a literal `"null"`) is fine.

Paired with `MatchExact`, this reproduces the fully explicit model some
APIs run (crpc-style): every version an endpoint exists at is declared,
nothing is inherited between versions, and `latest` is the one moving
pointer for callers that always want the newest. Callers on `latest`
accept behaviour changing under them; that's the trade the pseudo-version
has always been.

For custom dispatch built on the resolution half alone: `Resolve` returns
the bare sentinel `ErrLatest` for an enabled literal, and picking the
variant is yours to do.

## The preview overlay

`AllowPreview` names literals for the experimental channel, and the same
literal is how a preview variant is declared:

```go
api := version.NewResolver(version.Dates(),
    version.Header("X-API-Version"),
).AllowLatest("latest").AllowPreview("preview")

users := version.NewEndpoint(api)
users.Handle("2024-06-01", listUsersV2)
users.Handle("preview", listUsersNext) // at most one per endpoint
```

A request pinned `preview` is served by the preview variant. On endpoints
that don't declare one it falls back to the newest stable variant, even
under `MatchExact`, so a preview client follows latest wherever nothing
experimental is going on. Withdrawing an experiment is deleting the
variant; shipping it is registering the same handler at the date it went
stable.

Inside a handler, `Resolved.Preview` reports the ask. When the overlay
itself answered there is no version to name, so `Requested` and `Served`
stay the zero `V`. `Resolve` surfaces the ask as the bare sentinel
`ErrPreview`, mirroring `ErrLatest`.

## Reporting what answered

`StatusHeader` names a response header for `Serve` to set on successful
dispatch, in the spirit of crpc's `Infra-Endpoint-Status`:

```go
api := version.NewResolver(version.Dates(),
    version.Header("X-API-Version"),
).AllowLatest("latest").StatusHeader("Infra-Endpoint-Status")
```

The value is the channel the request asked for plus the version that
answered, when a dated variant did: `stable; version=2024-06-01`,
`latest; version=2024-06-01`, or a bare `preview` when the overlay ran.
It's informational, for an engineer eyeballing a response; nothing should
parse it. Off unless named, and absent on failures, which have their own
wire shape.

## Errors

Failures return `*dflhttp.ReqError` values, so the wire shape is dfl's
usual `{code, meta}` and a custom `Coercer` or `ErrorWriter` sees them like
any handler error. Each carries a sentinel for `errors.Is`, so
classification doesn't mean matching code strings:

| Code                   | Status | When                                   | Sentinel         |
| ---------------------- | ------ | -------------------------------------- | ---------------- |
| `version_missing`      | 400    | no source yielded a version            | `ErrMissing`     |
| `version_invalid`      | 400    | the value doesn't parse; meta carries it | `ErrInvalid`   |
| `version_unsupported`  | 400    | parsed, but no variant serves it; meta lists the supported versions | `ErrUnsupported` |
| `version_unconfigured` | 500    | the endpoint has no variants at all    |                  |

## Reading the version in a handler

`Serve` records the dispatch outcome on the context before calling the
variant:

```go
func listUsersV1(ctx context.Context, req *ListUsersReq) (*ListUsersResp, error) {
    resolved, _ := version.FromContext[time.Time](ctx)
    // resolved.Requested: the client's pin, after any Default
    // resolved.Served: this variant's version, older under MatchCompatible
    // resolved.Latest: the client asked for latest, not a concrete pin
    // resolved.Preview: the client asked for the preview overlay
}
```

Most variants never need it; it's there for behaviour gates finer than a
whole variant, and for logging which pins are still alive in the wild
before retiring one.

## Raw variants

`HandleFunc` mirrors `Router.HandleFunc` for variants the typed model
doesn't fit:

```go
users.HandleFunc("2024-06-01", func(w http.ResponseWriter, r *http.Request) error {
    // own the response outright
    return nil
})
```

And since `Serve` is just a `HandlerFunc`, versioned dispatch composes
anywhere one fits, not only at a route: wrap it in middleware, nest it,
or adapt it onto a bare `http.ServeMux` with a three-line error writer.

## Testing

An `Endpoint` needs no server and no Router to test: `Serve` takes a
recorder and a request. `Versions` reports what's registered, oldest
first, for asserting registration wiring:

```go
rec := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodGet, "/users", nil)
req.Header.Set("X-API-Version", "2024-03-15")

err := users.Serve(rec, req)
```

Variant handlers themselves stay plain functions; call them directly as
ever.
