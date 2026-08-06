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

	// ErrPreview: the request pinned a preview literal enabled with
	// AllowPreview. Like ErrLatest it is an outcome, not a failure:
	// Endpoint.Serve dispatches to the preview variant, or to the newest
	// stable one when the endpoint has no preview.
	ErrPreview = errors.New("version: request pinned preview")
)

// Resolver owns the two pluggable halves of versioning: the Scheme that
// gives versions meaning and the Sources that say where on a request the
// version travels. Build one per API and share it across every Endpoint,
// so the whole surface negotiates versions the same way.
type Resolver[V any] struct {
	scheme       Scheme[V]
	sources      []Source
	latest       []string
	preview      []string
	statusHeader string
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

// AllowLatest teaches the resolver request literals meaning "the newest
// variant of whichever endpoint answers", whatever the Match rule; pair
// with a trailing Default("latest") to make no-version requests mean the
// same. Literals match exactly, before the scheme parses; one the scheme
// could parse panics, as do blanks and duplicates. Calls accumulate, chain
// off NewResolver, and belong in wiring, before traffic.
func (rv *Resolver[V]) AllowLatest(literals ...string) *Resolver[V] {
	return rv.allowLiterals("latest", &rv.latest, literals)
}

// AllowPreview teaches the resolver request literals for the experimental
// overlay: a preview pin is served by the endpoint's preview variant when
// one is declared, and by the newest stable variant otherwise, so preview
// clients follow latest wherever nothing experimental is going on. The
// same literals declare a preview variant, Handle(e, "preview", h). The
// literal rules match AllowLatest's; a literal can't be both.
func (rv *Resolver[V]) AllowPreview(literals ...string) *Resolver[V] {
	return rv.allowLiterals("preview", &rv.preview, literals)
}

func (rv *Resolver[V]) allowLiterals(kind string, dst *[]string, literals []string) *Resolver[V] {
	if len(literals) == 0 {
		panic("dflhttp/version: enable at least one " + kind + " literal")
	}

	for _, l := range literals {
		if strings.TrimSpace(l) != l || l == "" {
			panic(fmt.Sprintf("dflhttp/version: blank or padded %s literal %q", kind, l))
		}

		if slices.Contains(rv.latest, l) || slices.Contains(rv.preview, l) {
			panic(fmt.Sprintf("dflhttp/version: literal %q is already enabled", l))
		}

		if _, err := rv.scheme.Parse(l); err == nil {
			panic(fmt.Sprintf("dflhttp/version: %s literal %q would shadow a version the scheme parses", kind, l))
		}

		*dst = append(*dst, l)
	}

	return rv
}

// StatusHeader names a response header for Endpoint.Serve to report how
// dispatch went: the channel the request asked for, plus the served
// version when a dated variant answered, e.g. "stable;
// version=2024-06-01". Informational, for the engineer reading the
// response; crpc callers will recognise "Infra-Endpoint-Status". Off
// unless set, and set once: naming a second header panics.
func (rv *Resolver[V]) StatusHeader(name string) *Resolver[V] {
	if strings.TrimSpace(name) == "" {
		panic("dflhttp/version: StatusHeader needs a header name")
	}

	if rv.statusHeader != "" {
		panic("dflhttp/version: StatusHeader is already set to " + rv.statusHeader)
	}

	rv.statusHeader = name

	return rv
}

// Resolve extracts and parses the request's version. Sources are tried in
// order and the first hit decides: a value that fails to parse is a 400,
// not a fall-through, because a garbled pin should be loud. Values are
// trimmed, whitespace-only is a miss, and latest and preview literals
// return bare ErrLatest and ErrPreview; other failures are *ReqError.
// Endpoint.Serve calls this for you; call it for custom dispatch.
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

		if slices.Contains(rv.preview, raw) {
			return zero, ErrPreview
		}

		v, err := rv.scheme.Parse(raw)
		if err != nil {
			return zero, dflhttp.Wrap(ErrInvalid, "version_invalid",
				dflhttp.M{"version": raw}, err)
		}

		return v, nil
	}

	return zero, dflhttp.Wrap(ErrMissing, "version_missing", nil)
}
