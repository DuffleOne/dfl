package version_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	dflhttp "github.com/duffleone/dfl/http"
	"github.com/duffleone/dfl/http/version"
)

// dateV3 sits past both handler variants; the withdrawal tests mark the
// endpoint gone from here.
const dateV3 = "2024-09-01"

// getPath is get for routes other than /users, which the registrar tests
// need since they declare several.
func getPath(t *testing.T, h http.Handler, target, pin string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, target, nil)
	if pin != "" {
		req.Header.Set("X-API-Version", pin)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// withdrawnEndpoint is datedEndpoint plus a withdrawal at dateV3 and a
// latest literal, the standard fixture for the 410 tests.
func withdrawnEndpoint(t *testing.T) http.Handler {
	t.Helper()

	api := version.NewResolver(version.Dates(), version.Header("X-API-Version")).
		AllowLatest("latest")

	users := version.NewEndpoint(api)
	users.Handle(dateV1, handleUserV1)
	users.Handle(dateV2, handleUserV2)
	users.Withdraw(dateV3)

	r := dflhttp.NewRouter(http.NewServeMux())
	r.HandleFunc(http.MethodGet, "/users", users.Serve)

	return r
}

// TestWithdrawCutsOffFromVersionOnward: pins before the withdrawal keep
// their variants, pins at or past it get 410 endpoint_withdrawn naming
// the withdrawal version, and a latest pin lands on the marker too.
func TestWithdrawCutsOffFromVersionOnward(t *testing.T) {
	h := withdrawnEndpoint(t)

	for _, pin := range []string{dateV1, dateV2} {
		if rec := get(t, h, pin); rec.Code != http.StatusOK {
			t.Errorf("pin %s: status = %d, want 200 (withdrawal must not touch older pins)", pin, rec.Code)
		}
	}

	for _, pin := range []string{dateV3, "2025-01-01", "latest"} {
		rec := get(t, h, pin)
		if rec.Code != http.StatusGone {
			t.Errorf("pin %s: status = %d, want 410", pin, rec.Code)

			continue
		}

		body := decodeErr(t, rec)
		if body.Code != "endpoint_withdrawn" {
			t.Errorf("pin %s: code = %q, want endpoint_withdrawn", pin, body.Code)
		}

		if got, _ := body.Meta["withdrawn_at"].(string); got != dateV3 {
			t.Errorf("pin %s: withdrawn_at = %q, want %s", pin, got, dateV3)
		}
	}
}

// TestWithdrawnVersionExcludedFromSupported: the marker must never be
// offered as a version to pin, so an unsupported error's metadata lists
// only the servable variants.
func TestWithdrawnVersionExcludedFromSupported(t *testing.T) {
	rec := get(t, withdrawnEndpoint(t), "2020-01-01")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 version_unsupported", rec.Code)
	}

	body := decodeErr(t, rec)

	var supported []string

	raw, _ := body.Meta["supported"].([]any)
	for _, v := range raw {
		s, _ := v.(string)
		supported = append(supported, s)
	}

	if !slices.Equal(supported, []string{dateV1, dateV2}) {
		t.Errorf("supported = %v, want [%s %s] without the withdrawal marker", supported, dateV1, dateV2)
	}
}

// TestWithdrawRejectsChannelLiterals: a withdrawal is a dated API change;
// pointing it at a channel literal is a wiring bug and panics.
func TestWithdrawRejectsChannelLiterals(t *testing.T) {
	api := version.NewResolver(version.Dates(), version.Header("X-API-Version")).
		AllowLatest("latest")

	e := version.NewEndpoint(api)

	defer func() {
		if recover() == nil {
			t.Error("expected Withdraw(\"latest\") to panic")
		}
	}()

	e.Withdraw("latest")
}

// TestWarnUnpinnedHeader: the warning header appears exactly when the
// Fallback answered for a versionless request, naming the served version,
// and never for a client that pinned.
func TestWarnUnpinnedHeader(t *testing.T) {
	api := version.NewResolver(version.Dates(), version.Header("X-API-Version")).
		AllowLatest("latest").
		Fallback("latest").
		WarnUnpinned("X-API-Version-Warning")

	users := version.NewEndpoint(api)
	users.Handle(dateV1, handleUserV1)
	users.Handle(dateV2, handleUserV2)

	r := dflhttp.NewRouter(http.NewServeMux())
	r.HandleFunc(http.MethodGet, "/users", users.Serve)

	unpinned := get(t, r, "")
	if unpinned.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 via the fallback", unpinned.Code)
	}

	if got := unpinned.Header().Get("X-API-Version-Warning"); got != dateV2 {
		t.Errorf("warning header = %q, want the served version %s", got, dateV2)
	}

	pinned := get(t, r, dateV1)
	if got := pinned.Header().Get("X-API-Version-Warning"); got != "" {
		t.Errorf("warning header = %q on a pinned request, want none", got)
	}
}

// TestFallbackValidatesAtWiring: a fallback that is neither a version nor
// an enabled literal panics at wiring time, not per request, and a second
// Fallback panics like a second StatusHeader.
func TestFallbackValidatesAtWiring(t *testing.T) {
	t.Run("unparseable", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected Fallback(\"banana\") to panic")
			}
		}()

		version.NewResolver(version.Dates(), version.Header("X-API-Version")).Fallback("banana")
	})

	t.Run("duplicate", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected a second Fallback to panic")
			}
		}()

		version.NewResolver(version.Dates(), version.Header("X-API-Version")).
			Fallback(dateV1).
			Fallback(dateV2)
	})
}

// registrarFixture declares two routes inline: /users with two variants
// and a withdrawal, /health with one variant.
func registrarFixture(t *testing.T) (*version.Registrar[time.Time], http.Handler) {
	t.Helper()

	api := version.NewResolver(version.Dates(), version.Header("X-API-Version"))

	r := dflhttp.NewRouter(http.NewServeMux())
	g := version.NewRegistrar(r, api)

	g.Handle(http.MethodGet, "/users", dateV1, handleUserV1)
	g.Handle(http.MethodGet, "/users", dateV2, handleUserV2)
	g.Handle(http.MethodGet, "/health", dateV1, handleUserV1)
	g.Withdraw(http.MethodGet, "/health", dateV3)

	return g, r
}

// TestRegistrarBuildsOneDispatcherPerRoute: declarations for one (method,
// path) share an Endpoint, versions dispatch after Build, and the second
// route is independent of the first.
func TestRegistrarBuildsOneDispatcherPerRoute(t *testing.T) {
	g, h := registrarFixture(t)
	g.Build()

	rec := get(t, h, dateV2)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	if body := rec.Body.String(); !strings.Contains(body, "first_name") {
		t.Errorf("body = %s, want the v2 shape", body)
	}

	if rec := get(t, h, dateV1); rec.Code != http.StatusOK {
		t.Errorf("v1 pin: status = %d, want 200", rec.Code)
	}

	if rec := getPath(t, h, "/health", dateV3); rec.Code != http.StatusGone {
		t.Errorf("withdrawn health pin: status = %d, want 410", rec.Code)
	}
}

// TestRegistrarLatestSpansRoutes: the API-wide latest is the newest
// version any route declared, withdrawals included.
func TestRegistrarLatestSpansRoutes(t *testing.T) {
	g, _ := registrarFixture(t)

	latest, ok := g.Latest()
	if !ok {
		t.Fatal("Latest: want a version, got none")
	}

	if got := latest.Format(time.DateOnly); got != dateV3 {
		t.Errorf("Latest = %s, want %s (the withdrawal is an API change)", got, dateV3)
	}
}

// TestRegistrarPanicsOnDuplicateDeclaration: the same (method, path,
// version) twice is a startup panic, not a silent shadow.
func TestRegistrarPanicsOnDuplicateDeclaration(t *testing.T) {
	g, _ := registrarFixture(t)

	defer func() {
		if recover() == nil {
			t.Error("expected a duplicate declaration to panic")
		}
	}()

	g.Handle(http.MethodGet, "/users", dateV1, handleUserV1)
}

// TestRegistrarFreezesAfterBuild: declaring after Build panics, as does
// building twice; registration is startup-only by contract.
func TestRegistrarFreezesAfterBuild(t *testing.T) {
	t.Run("declare after build", func(t *testing.T) {
		g, _ := registrarFixture(t)
		g.Build()

		defer func() {
			if recover() == nil {
				t.Error("expected Handle after Build to panic")
			}
		}()

		g.Handle(http.MethodGet, "/late", dateV1, handleUserV1)
	})

	t.Run("double build", func(t *testing.T) {
		g, _ := registrarFixture(t)
		g.Build()

		defer func() {
			if recover() == nil {
				t.Error("expected a second Build to panic")
			}
		}()

		g.Build()
	})
}
