package version

import (
	"context"

	dflhttp "github.com/duffleone/dfl/http"
)

// routeKey identifies one logical route: the underlying mux matches on
// method and path alone, so header-selected versions of one path must
// share a single registration.
type routeKey struct {
	method, path string
}

// Registrar collects versioned route declarations and registers exactly
// one dispatcher per (method, path) at Build. Declarations read inline,
// version next to handler, instead of three lines of Endpoint wiring per
// route; the Endpoint stays the primitive underneath, one lazily made per
// route with the shared resolver and opts. Startup-only, like all
// registration here: not safe concurrently, and frozen once built.
type Registrar[V any] struct {
	router   *dflhttp.Router
	resolver *Resolver[V]
	opts     []EndpointOption
	order    []routeKey
	routes   map[routeKey]*Endpoint[V]
	built    bool
}

// NewRegistrar builds a Registrar declaring routes on router, resolving
// through resolver, with opts applied to every Endpoint it creates. It
// panics when router or resolver is nil.
func NewRegistrar[V any](router *dflhttp.Router, resolver *Resolver[V], opts ...EndpointOption) *Registrar[V] {
	if router == nil {
		panic("dflhttp/version: NewRegistrar needs a Router")
	}

	if resolver == nil {
		panic("dflhttp/version: NewRegistrar needs a Resolver")
	}

	return &Registrar[V]{
		router:   router,
		resolver: resolver,
		opts:     opts,
		routes:   map[routeKey]*Endpoint[V]{},
	}
}

// Handle declares the variant at version raw for (method, path), with
// Endpoint.Handle's typed contract. Declaring the same (method, path,
// version) twice panics, at startup, instead of one variant silently
// shadowing the other.
func (g *Registrar[V]) Handle[Req, Resp any](method, path, raw string, handler func(context.Context, Req) (Resp, error)) {
	g.endpoint(method, path).Handle(raw, handler)
}

// HandleFunc is Handle for raw HandlerFuncs, mirroring Endpoint.HandleFunc.
func (g *Registrar[V]) HandleFunc(method, path, raw string, h dflhttp.HandlerFunc) {
	g.endpoint(method, path).HandleFunc(raw, h)
}

// Withdraw declares (method, path) withdrawn from version raw onward,
// per Endpoint.Withdraw.
func (g *Registrar[V]) Withdraw(method, path, raw string) {
	g.endpoint(method, path).Withdraw(raw)
}

// Endpoint returns the Endpoint behind (method, path), creating it if new,
// for anything the declaration shorthand doesn't cover.
func (g *Registrar[V]) Endpoint(method, path string) *Endpoint[V] {
	return g.endpoint(method, path)
}

func (g *Registrar[V]) endpoint(method, path string) *Endpoint[V] {
	if g.built {
		panic("dflhttp/version: Registrar is already built")
	}

	key := routeKey{method: method, path: path}

	e, ok := g.routes[key]
	if !ok {
		e = NewEndpoint(g.resolver, g.opts...)
		g.routes[key] = e
		g.order = append(g.order, key)
	}

	return e
}

// Build registers each collected Endpoint's Serve on the router, one
// dispatcher per route in declaration order, and freezes the Registrar:
// declaring after Build panics, as does a second Build.
func (g *Registrar[V]) Build() {
	if g.built {
		panic("dflhttp/version: Registrar is already built")
	}

	g.built = true

	for _, key := range g.order {
		g.router.HandleFunc(key.method, key.path, g.routes[key].Serve)
	}
}

// Latest returns the newest version declared across every route, Withdraw
// markers included, since a withdrawal is an API change at its version.
// ok is false while nothing dated has been declared. This is the API-wide
// latest: what an unpinned client is implicitly running against.
func (g *Registrar[V]) Latest() (V, bool) {
	var (
		latest V
		found  bool
	)

	for _, e := range g.routes {
		for _, v := range e.Versions() {
			if !found || g.resolver.scheme.Compare(v, latest) > 0 {
				latest, found = v, true
			}
		}
	}

	return latest, found
}
