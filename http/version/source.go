package version

import "net/http"

// Source extracts the raw version string from a request; ok reports
// whether it found one, and a Resolver tries its sources in order until
// the first hit. The built-ins cover headers, query parameters, path
// values, and a static default; anything else is a func of this shape (a
// build number from User-Agent, a claim from a verified token).
type Source func(r *http.Request) (raw string, ok bool)

// Header reads the version from the named request header. Absent or
// empty is a miss.
func Header(name string) Source {
	return func(r *http.Request) (string, bool) {
		v := r.Header.Get(name)

		return v, v != ""
	}
}

// Query reads the version from the named query parameter. Absent or
// empty is a miss.
func Query(name string) Source {
	return func(r *http.Request) (string, bool) {
		v := r.URL.Query().Get(name)

		return v, v != ""
	}
}

// PathValue reads the version from a path segment bound by the router,
// like {v} in "/api/{v}/users". Absent or empty is a miss.
func PathValue(name string) Source {
	return func(r *http.Request) (string, bool) {
		v := r.PathValue(name)

		return v, v != ""
	}
}

// A static default is not a Source: use Resolver.Fallback, which also lets
// the resolver tell a real pin from a fallback, feeding WarnUnpinned.
