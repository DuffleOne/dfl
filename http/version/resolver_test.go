package version_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dflhttp "github.com/duffleone/dfl/http"
	"github.com/duffleone/dfl/http/version"
)

func datedResolver(sources ...version.Source) *version.Resolver[time.Time] {
	return version.NewResolver(version.Dates(), sources...)
}

// TestResolveTriesSourcesInOrder checks the first source to yield a value
// wins, and later sources back it up when earlier ones miss.
func TestResolveTriesSourcesInOrder(t *testing.T) {
	rv := datedResolver(version.Header("X-API-Version"), version.Query("api_version"))

	r := httptest.NewRequest(http.MethodGet, "/?api_version=2024-01-02", nil)
	r.Header.Set("X-API-Version", dateV2)

	got, err := rv.Resolve(r)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if want := dateV2; got.Format(time.DateOnly) != want {
		t.Errorf("resolved = %s, want %s (the header, not the query)", got.Format(time.DateOnly), want)
	}

	got, err = rv.Resolve(httptest.NewRequest(http.MethodGet, "/?api_version=2024-01-02", nil))
	if err != nil {
		t.Fatalf("resolve without header: %v", err)
	}

	if want := dateV1; got.Format(time.DateOnly) != want {
		t.Errorf("resolved = %s, want %s (the query fallback)", got.Format(time.DateOnly), want)
	}
}

// TestResolveDefaultCatchesVersionlessRequests checks a trailing Default
// pins requests that carry no version at all.
func TestResolveDefaultCatchesVersionlessRequests(t *testing.T) {
	rv := datedResolver(version.Header("X-API-Version"), version.Default(dateV1))

	got, err := rv.Resolve(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if want := dateV1; got.Format(time.DateOnly) != want {
		t.Errorf("resolved = %s, want the default %s", got.Format(time.DateOnly), want)
	}
}

// TestResolveMissingIs400 checks a request with no version and no Default
// fails with version_missing, carrying ErrMissing for errors.Is.
func TestResolveMissingIs400(t *testing.T) {
	rv := datedResolver(version.Header("X-API-Version"))

	_, err := rv.Resolve(httptest.NewRequest(http.MethodGet, "/", nil))
	if !errors.Is(err, version.ErrMissing) {
		t.Fatalf("err = %v, want ErrMissing", err)
	}

	var reqErr *dflhttp.ReqError
	if !errors.As(err, &reqErr) {
		t.Fatalf("err = %v, want a *dflhttp.ReqError", err)
	}

	if reqErr.StatusCode != http.StatusBadRequest || reqErr.Code != "version_missing" {
		t.Errorf("got %d %s, want 400 version_missing", reqErr.StatusCode, reqErr.Code)
	}
}

// TestResolveInvalidDoesNotFallThrough pins the loud-failure rule: a value
// that fails to parse is a 400 even when a Default sits behind it, rather
// than being silently replaced.
func TestResolveInvalidDoesNotFallThrough(t *testing.T) {
	rv := datedResolver(version.Header("X-API-Version"), version.Default(dateV1))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Version", "banana")

	_, err := rv.Resolve(r)
	if !errors.Is(err, version.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}

	var reqErr *dflhttp.ReqError
	if !errors.As(err, &reqErr) {
		t.Fatalf("err = %v, want a *dflhttp.ReqError", err)
	}

	if reqErr.StatusCode != http.StatusBadRequest || reqErr.Code != "version_invalid" {
		t.Errorf("got %d %s, want 400 version_invalid", reqErr.StatusCode, reqErr.Code)
	}

	if got := reqErr.Meta["version"]; got != "banana" {
		t.Errorf("meta version = %v, want the raw value", got)
	}
}

// TestResolveTrimsAndSkipsWhitespace checks surrounding whitespace is
// trimmed before parsing and a whitespace-only value counts as a miss.
func TestResolveTrimsAndSkipsWhitespace(t *testing.T) {
	rv := datedResolver(version.Header("X-API-Version"), version.Default(dateV1))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Version", "  2024-06-01  ")

	got, err := rv.Resolve(r)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if want := dateV2; got.Format(time.DateOnly) != want {
		t.Errorf("resolved = %s, want %s", got.Format(time.DateOnly), want)
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Version", "   ")

	got, err = rv.Resolve(r)
	if err != nil {
		t.Fatalf("resolve with blank header: %v", err)
	}

	if want := dateV1; got.Format(time.DateOnly) != want {
		t.Errorf("resolved = %s, want the default %s", got.Format(time.DateOnly), want)
	}
}

// TestResolvePathValue checks the PathValue source reads router-bound path
// segments.
func TestResolvePathValue(t *testing.T) {
	rv := version.NewResolver(version.Sequential(), version.PathValue("v"))

	r := httptest.NewRequest(http.MethodGet, "/api/v2/users", nil)
	r.SetPathValue("v", "v2")

	got, err := rv.Resolve(r)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if got != 2 {
		t.Errorf("resolved = %d, want 2", got)
	}
}

// TestNewResolverPanicsOnMisconfiguration pins the fail-fast contract: a
// nil scheme or an empty source list is a startup panic, not a request-time
// surprise.
func TestNewResolverPanicsOnMisconfiguration(t *testing.T) {
	t.Run("nil scheme", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected NewResolver to panic on a nil Scheme")
			}
		}()

		version.NewResolver[time.Time](nil, version.Header("X-API-Version"))
	})

	t.Run("no sources", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected NewResolver to panic with no Sources")
			}
		}()

		version.NewResolver(version.Dates())
	})
}
