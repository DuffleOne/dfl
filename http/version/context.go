package version

import "context"

// Resolved records the outcome of version dispatch for one request.
// Requested is the pin the client asked for (after any Default); Served is
// the variant that answered, older than Requested under MatchCompatible.
// Latest and Preview report pseudo-version pins: under Latest the two
// versions are equal, and under Preview with a preview variant both stay
// the zero V, there being no version to name.
type Resolved[V any] struct {
	Requested V
	Served    V
	Latest    bool
	Preview   bool
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
