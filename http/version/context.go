package version

import "context"

// Resolved records the outcome of version dispatch for one request.
// Requested is the version the client asked for (after any Default);
// Served is the version of the variant that answered, which under
// MatchCompatible can be older than Requested. Latest reports that the
// client asked for the latest pseudo-version (a literal enabled with
// Resolver.AllowLatest) rather than a concrete pin; Requested then equals
// Served, the newest variant at dispatch time.
type Resolved[V any] struct {
	Requested V
	Served    V
	Latest    bool
}

type resolvedKey struct{}

// FromContext returns the Resolved for the request being served,
// available inside a variant handler and anything it calls. A variant
// that needs finer-grained behaviour than its registration version can
// branch on Requested directly. ok is false outside a versioned dispatch,
// and when V is not the dispatching Endpoint's version type.
func FromContext[V any](ctx context.Context) (Resolved[V], bool) {
	resolved, ok := ctx.Value(resolvedKey{}).(Resolved[V])

	return resolved, ok
}
