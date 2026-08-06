// Package version dispatches one route to different handlers by API
// version, so an endpoint can change shape over time without breaking the
// clients already pinned to its old behaviour.
//
// Three pieces plug together:
//
//   - A Scheme gives versions their type and their order: Dates for
//     "2024-06-01" pins, Sequential for "v1"/"v2", Ordered for any
//     cmp.Ordered type with a parse function, or your own implementation.
//   - Sources say where the version travels on a request: Header, Query,
//     PathValue, a trailing Default pin, or any func(r) (string, bool),
//     which covers app build numbers, JWT claims, per-client pins, and
//     whatever else.
//   - An Endpoint holds one handler per version and picks the variant for
//     the resolved version before the underlying function runs.
//
// A Resolver pairs a Scheme with Sources once per API; every Endpoint
// shares it. Variants are the same typed handlers the Router takes, each
// version free to have its own Req and Resp shapes, and Endpoint.Serve is
// a dflhttp.HandlerFunc, so a versioned endpoint registers on a Router
// like any raw route:
//
//	api := version.NewResolver(version.Dates(),
//	    version.Header("X-API-Version"),
//	)
//
//	users := version.NewEndpoint(api)
//	version.Handle(users, "2024-01-02", listUsersV1)
//	version.Handle(users, "2024-06-01", listUsersV2)
//
//	r.HandleFunc(http.MethodGet, "/users", users.Serve)
//
// Dispatch defaults to MatchCompatible: a request pinned to "2024-03-15"
// is served by the "2024-01-02" variant, the newest one not newer than
// the pin. Endpoints therefore version independently; register a new
// variant only on the endpoints whose behaviour actually changed.
//
// WithMatch(MatchExact) runs the stricter regime instead, where a pin
// must name a declared variant outright, and Resolver.AllowLatest adds an
// opt-in latest pseudo-version served by an endpoint's newest variant.
// Together they reproduce the fully explicit model: every version an
// endpoint exists at is declared, nothing carries between versions, and
// "latest" is the one moving pointer.
package version

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Scheme defines a version format: how raw strings from the wire parse
// into V, how two V values order, and how a V renders back into a string
// for error metadata and introspection.
//
// Implementations must be comparison-consistent: Compare is called with
// values produced by Parse, and a value must compare equal to itself
// after a Parse/Format round trip.
type Scheme[V any] interface {
	// Parse turns a raw wire value into a version. The error is surfaced
	// to the client (inside a 400 version_invalid response), so it should
	// say what a well-formed version looks like rather than reference
	// internals.
	Parse(raw string) (V, error)

	// Compare orders two versions: negative when a is older than b, zero
	// when equal, positive when a is newer.
	Compare(a, b V) int

	// Format renders v canonically.
	Format(v V) string
}

// Dates returns the Scheme for date-stamped versions like "2024-06-01",
// the Stripe model: every breaking change gets the date it shipped, and
// clients pin the date they integrated against. Values parse with
// time.DateOnly and order chronologically.
func Dates() Scheme[time.Time] {
	return dateScheme{}
}

type dateScheme struct{}

func (dateScheme) Parse(raw string) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("not a YYYY-MM-DD date: %q", raw)
	}

	return t, nil
}

func (dateScheme) Compare(a, b time.Time) int {
	return a.Compare(b)
}

func (dateScheme) Format(v time.Time) string {
	return v.Format(time.DateOnly)
}

// Sequential returns the Scheme for counted versions: "v1", "v2", "v3".
// The leading v is optional and case-insensitive on the wire, so "2" and
// "V2" both mean version two; Format always renders it.
func Sequential() Scheme[int] {
	return seqScheme{}
}

type seqScheme struct{}

func (seqScheme) Parse(raw string) (int, error) {
	digits := strings.TrimPrefix(strings.ToLower(raw), "v")
	if digits == "" {
		return 0, fmt.Errorf("not a version number like v2: %q", raw)
	}

	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a version number like v2: %q", raw)
		}
	}

	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("not a version number like v2: %q", raw)
	}

	return n, nil
}

func (seqScheme) Compare(a, b int) int {
	return cmp.Compare(a, b)
}

func (seqScheme) Format(v int) string {
	return "v" + strconv.Itoa(v)
}

// Ordered builds a Scheme for any ordered type from just a parse
// function: cmp.Compare orders the values and fmt.Sprint formats them.
// This is the shortcut for versions that are naturally an int, float, or
// string, like an app build number:
//
//	version.Ordered(strconv.Atoi)
//
// Anything needing its own ordering or rendering (semver, say, where
// "v10" must sort above "v9" component-wise) implements Scheme directly
// instead.
func Ordered[V cmp.Ordered](parse func(raw string) (V, error)) Scheme[V] {
	return orderedScheme[V]{parse: parse}
}

type orderedScheme[V cmp.Ordered] struct {
	parse func(string) (V, error)
}

func (s orderedScheme[V]) Parse(raw string) (V, error) {
	return s.parse(raw)
}

func (orderedScheme[V]) Compare(a, b V) int {
	return cmp.Compare(a, b)
}

func (orderedScheme[V]) Format(v V) string {
	return fmt.Sprint(v)
}
