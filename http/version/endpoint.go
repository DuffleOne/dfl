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

// Endpoint is one logical route with a handler per version: register
// variants with Handle or HandleFunc, register Serve on the Router, and
// each request runs the variant the Match rule picks for its resolved
// version. Set up at startup like the Router itself: registration is not
// safe to run concurrently with Serve.
type Endpoint[V any] struct {
	resolver *Resolver[V]
	config   endpointConfig
	variants []variant[V]        // ascending by the resolver scheme's order
	preview  dflhttp.HandlerFunc // the experimental overlay, nil when none
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
// The contract matches Router.Handle: each variant has its own Req and
// Resp, binding is prepared once here, and misuse (an unparseable or
// duplicate version, an unbindable Req) panics at registration.
func (e *Endpoint[V]) Handle[Req, Resp any](raw string, handler func(context.Context, Req) (Resp, error)) {
	h, err := dflhttp.Adapt(e.config.parser, handler)
	if err != nil {
		panic("dflhttp/version: " + err.Error())
	}

	e.HandleFunc(raw, h)
}

// HandleFunc registers a raw HandlerFunc as the variant at version raw,
// for variants the typed model doesn't fit, mirroring Router.HandleFunc;
// it panics on an unparseable or duplicate version. When raw is a preview
// literal (Resolver.AllowPreview) the handler becomes the endpoint's
// preview variant instead of a dated one; at most one may be declared.
func (e *Endpoint[V]) HandleFunc(raw string, h dflhttp.HandlerFunc) {
	if slices.Contains(e.resolver.preview, raw) {
		if e.preview != nil {
			panic("dflhttp/version: duplicate preview variant")
		}

		e.preview = h

		return
	}

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
// rule, records the outcome on the context for FromContext, and calls the
// variant. It's a dflhttp.HandlerFunc, so it registers as a raw route. A
// latest pin gets the newest variant, whatever the Match rule; a preview
// pin gets the preview variant, or the newest without one. Failures are
// *dflhttp.ReqError; an Endpoint with no variants is a 500 version_unconfigured.
func (e *Endpoint[V]) Serve(w http.ResponseWriter, r *http.Request) error {
	if len(e.variants) == 0 && e.preview == nil {
		// Not the caller's fault, so this one opts out of ReqError's 400
		// default: an Endpoint with nothing registered is a wiring bug.
		return dflhttp.New("version_unconfigured", nil).
			WithStatus(http.StatusInternalServerError)
	}

	requested, err := e.resolver.Resolve(r)

	var latest, preview bool

	switch {
	case errors.Is(err, ErrLatest):
		latest = true
	case errors.Is(err, ErrPreview):
		preview = true
	case err != nil:
		return err
	}

	var (
		chosen variant[V]
		dated  = true
	)

	switch {
	case preview && e.preview != nil:
		// The preview variant has no version of its own: chosen keeps the
		// zero V and only the flag travels.
		chosen.handle = e.preview
		dated = false
	case latest || preview:
		if len(e.variants) == 0 {
			return e.unsupported(channel(latest, preview))
		}

		chosen = e.variants[len(e.variants)-1]
		requested = chosen.version
	default:
		var ok bool

		chosen, ok = e.pick(requested)
		if !ok {
			return e.unsupported(e.resolver.scheme.Format(requested))
		}
	}

	if name := e.resolver.statusHeader; name != "" {
		value := channel(latest, preview)
		if dated {
			value += "; version=" + e.resolver.scheme.Format(chosen.version)
		}

		w.Header().Set(name, value)
	}

	resolved := Resolved[V]{Requested: requested, Served: chosen.version, Latest: latest, Preview: preview}
	r = r.WithContext(context.WithValue(r.Context(), resolvedKey{}, resolved))

	return chosen.handle(w, r)
}

// unsupported builds the 400 version_unsupported ReqError, naming what the
// request asked for and every version that would have worked.
func (e *Endpoint[V]) unsupported(asked string) error {
	return dflhttp.Wrap(ErrUnsupported, "version_unsupported", dflhttp.M{
		"version":   asked,
		"supported": e.supported(),
	})
}

// channel names the dispatch channel a request asked for, for the status
// header and error metadata. The zero case is a concrete pin: "stable".
func channel(latest, preview bool) string {
	switch {
	case preview:
		return "preview"
	case latest:
		return "latest"
	default:
		return "stable"
	}
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
