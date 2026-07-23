package http_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	dflhttp "github.com/duffleone/dfl/http"
	"github.com/duffleone/dfl/http/internal/routertest"
)

// TestServeMuxBackend runs the router conformance suite over stdlib
// *http.ServeMux, exercising the PatternMux registration path.
func TestServeMuxBackend(t *testing.T) {
	routertest.Run(t, routertest.Factory{
		New: func(opts ...dflhttp.Option) (*dflhttp.Router, http.Handler) {
			r := dflhttp.NewRouter(http.NewServeMux(), opts...)

			return r, r
		},
	})
}

// TestChiBackend runs the same suite over *chi.Mux, exercising the
// MethodMux registration path. The two backends must be indistinguishable
// from a handler's point of view; this is the test that enforces it.
func TestChiBackend(t *testing.T) {
	routertest.Run(t, routertest.Factory{
		New: func(opts ...dflhttp.Option) (*dflhttp.Router, http.Handler) {
			r := dflhttp.NewRouter(chi.NewMux(), opts...)

			return r, r
		},
	})
}

// TestNewRouterRejectsUnknownMux pins the constructor contract: a mux that
// satisfies neither MethodMux nor PatternMux panics at construction, not at
// first request.
func TestNewRouterRejectsUnknownMux(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected NewRouter to panic on a mux with no registration method")
		}
	}()

	dflhttp.NewRouter(bareMux{})
}

// bareMux is an http.Handler with no registration method at all.
type bareMux struct{}

func (bareMux) ServeHTTP(http.ResponseWriter, *http.Request) {}
