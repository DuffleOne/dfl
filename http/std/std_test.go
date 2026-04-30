package std_test

import (
	"net/http"
	"testing"

	dflhttp "github.com/duffleone/dfl/http"
	"github.com/duffleone/dfl/http/internal/routertest"
	dflstd "github.com/duffleone/dfl/http/std"
)

// TestConformance runs the shared router conformance suite against the
// stdlib backend. The suite itself lives in http/internal/routertest so
// every backend is verified by the same test code.
func TestConformance(t *testing.T) {
	routertest.Run(t, routertest.Factory{
		New: func() (dflhttp.Router, http.Handler) {
			r := dflstd.New()

			return r, r
		},
		NewWithCoercer: func(c dflhttp.Coercer) (dflhttp.Router, http.Handler) {
			r := dflstd.New(dflstd.WithCoercer(c))

			return r, r
		},
	})
}
