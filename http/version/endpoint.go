package version

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	dflhttp "github.com/duffleone/dfl/http"
)

// Match picks the dispatch rule for an Endpoint.
type Match int

const (
	// MatchCompatible serves the newest variant that is not newer than
	// the requested version. A client pinned to a version keeps its
	// behaviour as newer variants land, and endpoints stay sparse: a
	// variant is registered only where behaviour changed. A version older
	// than every variant is rejected as unsupported.
	MatchCompatible Match = iota

	// MatchExact serves only a variant whose version equals the requested
	// version outright; anything else is unsupported.
	MatchExact
)

// EndpointOption configures a NewEndpoint call.
type EndpointOption func(*endpointConfig)

type endpointConfig struct {
	match  Match
	parser *dflhttp.RequestParser
}

// WithMatch sets the dispatch rule. The default is MatchCompatible.
func WithMatch(m Match) EndpointOption {
	return func(c *endpointConfig) {
		c.match = m
	}
}

// WithParser sets the RequestParser typed variants bind through,
// mirroring dflhttp.WithRequestParser on the Router. The default is
// DefaultRequestParser.
func WithParser(p *dflhttp.RequestParser) EndpointOption {
	return func(c *endpointConfig) {
		c.parser = p
	}
}

// variant pairs a parsed version with the handler that serves it.
type variant[V any] struct {
	version V
	handle  dflhttp.HandlerFunc
}

// Endpoint is one logical route with a handler per version. Register
// variants with Handle or HandleFunc, then register Serve on the Router;
// each request resolves its version through the shared Resolver and runs
// the variant the Match rule picks.
//
// Like the Router, an Endpoint is set up once at startup: register every
// variant before serving traffic, registration is not safe to run
// concurrently with Serve.
type Endpoint[V any] struct {
	resolver *Resolver[V]
	config   endpointConfig
	variants []variant[V] // ascending by the resolver scheme's order
}

// NewEndpoint builds an empty Endpoint dispatching through resolver. It
// panics when resolver is nil.
func NewEndpoint[V any](resolver *Resolver[V], opts ...EndpointOption) *Endpoint[V] {
	if resolver == nil {
		panic("dflhttp/version: NewEndpoint needs a Resolver")
	}

	var config endpointConfig

	for _, opt := range opts {
		opt(&config)
	}

	return &Endpoint[V]{resolver: resolver, config: config}
}

// Handle registers a typed handler on e as the variant at version raw.
// The contract matches Router.Handle: the handler shape is
// func(ctx, *Req) (*Resp, error), each variant free to have its own Req
// and Resp, binding is prepared once here, and misuse panics at
// registration rather than on the first request: a version the scheme
// can't parse, a duplicate version, or a Req the parser can't bind.
//
// It's a package function rather than an Endpoint method because a method
// carrying its own type parameters on an already-generic receiver
// produces export data golangci-lint's importer can't read yet, the same
// tooling lag .golangci.yml documents. Fold it into a method once the
// toolchain catches up.
func Handle[V, Req, Resp any](e *Endpoint[V], raw string, handler func(context.Context, Req) (Resp, error)) {
	h, err := dflhttp.Adapt(e.config.parser, handler)
	if err != nil {
		panic("dflhttp/version: " + err.Error())
	}

	e.HandleFunc(raw, h)
}

// HandleFunc registers a raw HandlerFunc as the variant at version raw,
// for variants the typed model doesn't fit; it mirrors Router.HandleFunc.
// It panics on a version the scheme can't parse or a duplicate version.
func (e *Endpoint[V]) HandleFunc(raw string, h dflhttp.HandlerFunc) {
	v, err := e.resolver.scheme.Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("dflhttp/version: bad version %q: %v", raw, err))
	}

	at, found := e.search(v)
	if found {
		panic("dflhttp/version: duplicate variant at " + e.resolver.scheme.Format(v))
	}

	e.variants = slices.Insert(e.variants, at, variant[V]{version: v, handle: h})
}

// Serve resolves the request's version, picks the variant per the Match
// rule, records the outcome on the context for FromContext, and calls
// the variant. Its shape is dflhttp.HandlerFunc, so an Endpoint registers
// on a Router as a raw route:
//
//	r.HandleFunc(http.MethodGet, "/users", users.Serve)
//
// A request pinned to a latest literal (Resolver.AllowLatest) is served
// by the newest registered variant, whatever the Match rule.
//
// Failures return *dflhttp.ReqError values (see the package errors), so
// they flow through the Router's usual error pipeline. Serving an
// Endpoint with no variants is a 500 version_unconfigured.
func (e *Endpoint[V]) Serve(w http.ResponseWriter, r *http.Request) error {
	if len(e.variants) == 0 {
		return dflhttp.New(http.StatusInternalServerError, "version_unconfigured", nil)
	}

	requested, err := e.resolver.Resolve(r)

	latest := errors.Is(err, ErrLatest)
	if err != nil && !latest {
		return err
	}

	var chosen variant[V]

	if latest {
		chosen = e.variants[len(e.variants)-1]
		requested = chosen.version
	} else {
		var ok bool

		chosen, ok = e.pick(requested)
		if !ok {
			return dflhttp.Wrap(ErrUnsupported, http.StatusBadRequest, "version_unsupported", dflhttp.M{
				"version":   e.resolver.scheme.Format(requested),
				"supported": e.supported(),
			})
		}
	}

	resolved := Resolved[V]{Requested: requested, Served: chosen.version, Latest: latest}
	r = r.WithContext(context.WithValue(r.Context(), resolvedKey{}, resolved))

	return chosen.handle(w, r)
}

// pick applies the Match rule to an already-resolved version.
func (e *Endpoint[V]) pick(requested V) (variant[V], bool) {
	at, found := e.search(requested)
	if found {
		return e.variants[at], true
	}

	if e.config.match == MatchExact || at == 0 {
		return variant[V]{}, false
	}

	return e.variants[at-1], true
}

// search binary-searches the ascending variants for v under the scheme's
// order, with BinarySearchFunc's contract: the index of v when found is
// true, otherwise v's insertion point.
func (e *Endpoint[V]) search(v V) (int, bool) {
	return slices.BinarySearchFunc(e.variants, v, func(x variant[V], target V) int {
		return e.resolver.scheme.Compare(x.version, target)
	})
}

// Versions returns the registered variant versions, oldest first.
func (e *Endpoint[V]) Versions() []V {
	out := make([]V, len(e.variants))
	for i, va := range e.variants {
		out[i] = va.version
	}

	return out
}

// supported renders every registered version for error metadata.
func (e *Endpoint[V]) supported() []string {
	out := make([]string, len(e.variants))
	for i, va := range e.variants {
		out[i] = e.resolver.scheme.Format(va.version)
	}

	return out
}
