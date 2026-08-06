package version

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	dflhttp "github.com/duffleone/dfl/http"
)

// Sentinel errors recorded as reasons on the *dflhttp.ReqError values this
// package returns, so a Coercer or ErrorWriter can classify them with
// errors.Is instead of matching code strings.
var (
	// ErrMissing: no source yielded a version and the resolver has no
	// Default. On the wire: 400 version_missing.
	ErrMissing = errors.New("version: no version on the request")

	// ErrInvalid: a source yielded a value the scheme can't parse. On the
	// wire: 400 version_invalid.
	ErrInvalid = errors.New("version: version does not parse")

	// ErrUnsupported: the version parsed, but no variant serves it. On
	// the wire: 400 version_unsupported.
	ErrUnsupported = errors.New("version: no variant serves this version")

	// ErrLatest: the request pinned a latest literal enabled with
	// AllowLatest. Not a failure: Endpoint.Serve catches it and dispatches
	// to its newest variant. Resolve surfaces it so custom dispatch built
	// on the resolution half alone can do the same; it is deliberately not
	// a *dflhttp.ReqError, so if it ever reaches a Router unhandled it
	// falls through as a 500 rather than masquerading as a client error.
	ErrLatest = errors.New("version: request pinned latest")
)

// Resolver owns the two pluggable halves of versioning: the Scheme that
// gives versions meaning and the Sources that say where on a request the
// version travels. Build one per API and share it across every Endpoint,
// so the whole surface negotiates versions the same way.
type Resolver[V any] struct {
	scheme  Scheme[V]
	sources []Source
	latest  []string
}

// NewResolver builds a Resolver from a scheme and an ordered list of
// sources. It panics when scheme is nil or sources is empty, mirroring
// NewRouter's fail-fast on misconfiguration.
func NewResolver[V any](scheme Scheme[V], sources ...Source) *Resolver[V] {
	if scheme == nil {
		panic("dflhttp/version: NewResolver needs a Scheme")
	}

	if len(sources) == 0 {
		panic("dflhttp/version: NewResolver needs at least one Source")
	}

	return &Resolver[V]{scheme: scheme, sources: sources}
}

// AllowLatest teaches the resolver request literals that mean "the newest
// variant of whichever endpoint answers". With AllowLatest("latest"), a
// request pinned to latest is served by the endpoint's newest registered
// variant, whatever the Match rule; combined with a trailing
// Default("latest") source, requests carrying no version at all mean the
// same. Leave it unconfigured to keep pins concrete, the package default.
//
// Literals are matched exactly, after the usual whitespace trim, and are
// checked before the scheme parses. A literal the scheme could itself
// parse panics, since it would shadow a real version, as do an empty call,
// a blank literal, and a duplicate. Calls accumulate and belong in wiring,
// before traffic; it returns the resolver so it can chain off NewResolver.
func (rv *Resolver[V]) AllowLatest(literals ...string) *Resolver[V] {
	if len(literals) == 0 {
		panic("dflhttp/version: AllowLatest needs at least one literal")
	}

	for _, l := range literals {
		if strings.TrimSpace(l) != l || l == "" {
			panic(fmt.Sprintf("dflhttp/version: blank or padded latest literal %q", l))
		}

		if slices.Contains(rv.latest, l) {
			panic("dflhttp/version: duplicate latest literal " + l)
		}

		if _, err := rv.scheme.Parse(l); err == nil {
			panic(fmt.Sprintf("dflhttp/version: latest literal %q would shadow a version the scheme parses", l))
		}

		rv.latest = append(rv.latest, l)
	}

	return rv
}

// Resolve extracts and parses the request's version. Sources are tried
// in order and the first one to yield a value decides the request: a
// value that fails to parse is a 400, not a fall-through to the next
// source, because a garbled explicit version should be loud rather than
// silently defaulted. Values are trimmed of surrounding whitespace, and
// a whitespace-only value counts as a miss. A value matching an
// AllowLatest literal returns ErrLatest, bare, for the caller to act on.
//
// Errors are otherwise *dflhttp.ReqError values carrying ErrMissing or
// ErrInvalid as causes, so they can go straight back through a Router's
// error pipeline. Endpoint.Serve calls Resolve for you; call it yourself
// only when building something custom on the resolution half alone.
func (rv *Resolver[V]) Resolve(r *http.Request) (V, error) {
	var zero V

	for _, source := range rv.sources {
		raw, ok := source(r)
		if !ok {
			continue
		}

		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		if slices.Contains(rv.latest, raw) {
			return zero, ErrLatest
		}

		v, err := rv.scheme.Parse(raw)
		if err != nil {
			return zero, dflhttp.Wrap(ErrInvalid, http.StatusBadRequest, "version_invalid",
				dflhttp.M{"version": raw}, err)
		}

		return v, nil
	}

	return zero, dflhttp.Wrap(ErrMissing, http.StatusBadRequest, "version_missing", nil)
}
