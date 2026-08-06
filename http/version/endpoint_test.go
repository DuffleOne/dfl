package version_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dflhttp "github.com/duffleone/dfl/http"
	"github.com/duffleone/dfl/http/version"
)

const (
	dateV1 = "2024-01-02"
	dateV2 = "2024-06-01"
)

// userV1 and userV2 are deliberately different response shapes: the point
// of the package is that variants of one endpoint evolve independently.
type userV1 struct {
	Name string `json:"name"`
}

type userV2 struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func handleUserV1(context.Context, *dflhttp.Empty) (*userV1, error) {
	return &userV1{Name: "Ada Lovelace"}, nil
}

func handleUserV2(context.Context, *dflhttp.Empty) (*userV2, error) {
	return &userV2{FirstName: "Ada", LastName: "Lovelace"}, nil
}

// datedEndpoint builds the standard two-variant endpoint the dispatch
// tests share, registered on a real Router over *http.ServeMux.
func datedEndpoint(t *testing.T, opts ...version.EndpointOption) http.Handler {
	t.Helper()

	api := version.NewResolver(version.Dates(), version.Header("X-API-Version"))

	users := version.NewEndpoint(api, opts...)
	version.Handle(users, dateV1, handleUserV1)
	version.Handle(users, dateV2, handleUserV2)

	r := dflhttp.NewRouter(http.NewServeMux())
	r.HandleFunc(http.MethodGet, "/users", users.Serve)

	return r
}

// get drives one request through the handler with the given version pin;
// an empty pin sends no version at all.
func get(t *testing.T, h http.Handler, pin string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	if pin != "" {
		r.Header.Set("X-API-Version", pin)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	return rec
}

type errBody struct {
	Code       string         `json:"code"`
	StatusCode int            `json:"status_code"`
	Meta       map[string]any `json:"meta"`
}

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) errBody {
	t.Helper()

	var body errBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}

	return body
}

// TestCompatibleServesNewestNotNewer is the dispatch rule itself. A pin on
// a variant gets that variant; a pin between variants gets the older one;
// a pin after every variant gets the newest.
func TestCompatibleServesNewestNotNewer(t *testing.T) {
	h := datedEndpoint(t)

	tests := []struct {
		pin  string
		want string
	}{
		{pin: dateV1, want: `{"name":"Ada Lovelace"}`},
		{pin: "2024-03-15", want: `{"name":"Ada Lovelace"}`},
		{pin: dateV2, want: `{"first_name":"Ada","last_name":"Lovelace"}`},
		{pin: "2025-01-01", want: `{"first_name":"Ada","last_name":"Lovelace"}`},
	}

	for _, tt := range tests {
		rec := get(t, h, tt.pin)
		if rec.Code != http.StatusOK {
			t.Errorf("pin %s: status = %d, want 200", tt.pin, rec.Code)

			continue
		}

		if got := strings.TrimSpace(rec.Body.String()); got != tt.want {
			t.Errorf("pin %s: body = %s, want %s", tt.pin, got, tt.want)
		}
	}
}

// TestPinOlderThanEveryVariantIsUnsupported checks the other edge of
// MatchCompatible: a client older than the endpoint itself is a 400 with
// the supported versions in the meta.
func TestPinOlderThanEveryVariantIsUnsupported(t *testing.T) {
	rec := get(t, datedEndpoint(t), "2023-01-01")

	body := decodeErr(t, rec)
	if rec.Code != http.StatusBadRequest || body.Code != "version_unsupported" {
		t.Fatalf("got %d %s, want 400 version_unsupported", rec.Code, body.Code)
	}

	supported, ok := body.Meta["supported"].([]any)
	if !ok || len(supported) != 2 {
		t.Fatalf("meta supported = %v, want both registered versions", body.Meta["supported"])
	}

	if supported[0] != dateV1 || supported[1] != dateV2 {
		t.Errorf("meta supported = %v, want [%s %s]", supported, dateV1, dateV2)
	}
}

// TestExactMatchTakesNoNeighbours checks WithMatch(MatchExact): only a pin
// naming a variant outright is served.
func TestExactMatchTakesNoNeighbours(t *testing.T) {
	h := datedEndpoint(t, version.WithMatch(version.MatchExact))

	if rec := get(t, h, dateV2); rec.Code != http.StatusOK {
		t.Errorf("exact pin: status = %d, want 200", rec.Code)
	}

	rec := get(t, h, "2024-03-15")

	body := decodeErr(t, rec)
	if rec.Code != http.StatusBadRequest || body.Code != "version_unsupported" {
		t.Errorf("between pin: got %d %s, want 400 version_unsupported", rec.Code, body.Code)
	}
}

// TestLatestServesNewestVariant covers the latest pseudo-version: an
// enabled literal is served by the newest variant even under MatchExact,
// and a trailing Default("latest") gives versionless requests the same
// meaning. Without AllowLatest, "latest" stays an invalid version.
func TestLatestServesNewestVariant(t *testing.T) {
	newest := `{"first_name":"Ada","last_name":"Lovelace"}`

	t.Run("literal pin under MatchExact", func(t *testing.T) {
		api := version.NewResolver(version.Dates(),
			version.Header("X-API-Version"),
		).AllowLatest("latest")

		users := version.NewEndpoint(api, version.WithMatch(version.MatchExact))
		version.Handle(users, dateV1, handleUserV1)
		version.Handle(users, dateV2, handleUserV2)

		r := dflhttp.NewRouter(http.NewServeMux())
		r.HandleFunc(http.MethodGet, "/users", users.Serve)

		rec := get(t, r, "latest")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
		}

		if got := strings.TrimSpace(rec.Body.String()); got != newest {
			t.Errorf("body = %s, want the newest variant %s", got, newest)
		}
	})

	t.Run("versionless requests via Default", func(t *testing.T) {
		api := version.NewResolver(version.Dates(),
			version.Header("X-API-Version"),
			version.Default("latest"),
		).AllowLatest("latest")

		users := version.NewEndpoint(api)
		version.Handle(users, dateV1, handleUserV1)
		version.Handle(users, dateV2, handleUserV2)

		r := dflhttp.NewRouter(http.NewServeMux())
		r.HandleFunc(http.MethodGet, "/users", users.Serve)

		rec := get(t, r, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
		}

		if got := strings.TrimSpace(rec.Body.String()); got != newest {
			t.Errorf("body = %s, want the newest variant %s", got, newest)
		}
	})

	t.Run("disabled by default", func(t *testing.T) {
		rec := get(t, datedEndpoint(t), "latest")

		body := decodeErr(t, rec)
		if rec.Code != http.StatusBadRequest || body.Code != "version_invalid" {
			t.Errorf("got %d %s, want 400 version_invalid", rec.Code, body.Code)
		}
	})
}

// TestLatestIsRecordedOnTheContext checks a variant serving a latest pin
// sees Latest set and Requested equal to Served.
func TestLatestIsRecordedOnTheContext(t *testing.T) {
	api := version.NewResolver(version.Dates(),
		version.Header("X-API-Version"),
	).AllowLatest("latest")

	type pinInfo struct {
		Requested string `json:"requested"`
		Served    string `json:"served"`
		Latest    bool   `json:"latest"`
	}

	e := version.NewEndpoint(api)
	version.Handle(e, dateV1, func(ctx context.Context, _ *dflhttp.Empty) (*pinInfo, error) {
		resolved, ok := version.FromContext[time.Time](ctx)
		if !ok {
			return nil, errors.New("no Resolved on the context")
		}

		return &pinInfo{
			Requested: resolved.Requested.Format(time.DateOnly),
			Served:    resolved.Served.Format(time.DateOnly),
			Latest:    resolved.Latest,
		}, nil
	})

	r := dflhttp.NewRouter(http.NewServeMux())
	r.HandleFunc(http.MethodGet, "/users", e.Serve)

	rec := get(t, r, "latest")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	var info pinInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !info.Latest || info.Requested != dateV1 || info.Served != dateV1 {
		t.Errorf("resolved = %+v, want latest with requested and served both %s", info, dateV1)
	}
}

// TestMissingAndInvalidFlowThroughTheRouter checks resolver failures come
// out of the Router's error pipeline as dfl's wire shape.
func TestMissingAndInvalidFlowThroughTheRouter(t *testing.T) {
	h := datedEndpoint(t)

	body := decodeErr(t, get(t, h, ""))
	if body.Code != "version_missing" || body.StatusCode != http.StatusBadRequest {
		t.Errorf("no pin: got %+v, want 400 version_missing", body)
	}

	body = decodeErr(t, get(t, h, "banana"))
	if body.Code != "version_invalid" || body.StatusCode != http.StatusBadRequest {
		t.Errorf("bad pin: got %+v, want 400 version_invalid", body)
	}
}

// TestResolvedIsOnTheContext checks a variant can read the dispatch
// outcome: the client's requested pin and the version actually served.
func TestResolvedIsOnTheContext(t *testing.T) {
	api := version.NewResolver(version.Dates(), version.Header("X-API-Version"))

	type pinInfo struct {
		Requested string `json:"requested"`
		Served    string `json:"served"`
	}

	e := version.NewEndpoint(api)
	version.Handle(e, dateV1, func(ctx context.Context, _ *dflhttp.Empty) (*pinInfo, error) {
		resolved, ok := version.FromContext[time.Time](ctx)
		if !ok {
			return nil, errors.New("no Resolved on the context")
		}

		return &pinInfo{
			Requested: resolved.Requested.Format(time.DateOnly),
			Served:    resolved.Served.Format(time.DateOnly),
		}, nil
	})

	r := dflhttp.NewRouter(http.NewServeMux())
	r.HandleFunc(http.MethodGet, "/users", e.Serve)

	rec := get(t, r, "2024-03-15")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	var info pinInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if info.Requested != "2024-03-15" || info.Served != dateV1 {
		t.Errorf("resolved = %+v, want requested 2024-03-15 served %s", info, dateV1)
	}
}

// TestHandleFuncRegistersRawVariants checks the escape hatch: a variant
// that isn't a typed handler still dispatches by version.
func TestHandleFuncRegistersRawVariants(t *testing.T) {
	api := version.NewResolver(version.Sequential(), version.Query("v"), version.Default("v1"))

	e := version.NewEndpoint(api)
	e.HandleFunc("v1", func(w http.ResponseWriter, _ *http.Request) error {
		_, err := io.WriteString(w, "one")

		return err
	})
	e.HandleFunc("v2", func(w http.ResponseWriter, _ *http.Request) error {
		_, err := io.WriteString(w, "two")

		return err
	})

	r := dflhttp.NewRouter(http.NewServeMux())
	r.HandleFunc(http.MethodGet, "/thing", e.Serve)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/thing?v=2", nil))

	if got := rec.Body.String(); got != "two" {
		t.Errorf("v=2 body = %q, want two", got)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/thing", nil))

	if got := rec.Body.String(); got != "one" {
		t.Errorf("default body = %q, want one", got)
	}
}

// TestServeWithNoVariantsIs500 checks an endpoint nothing was registered
// on reports a server-side misconfiguration, not a client error.
func TestServeWithNoVariantsIs500(t *testing.T) {
	api := version.NewResolver(version.Dates(), version.Header("X-API-Version"))
	e := version.NewEndpoint(api)

	err := e.Serve(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var reqErr *dflhttp.ReqError
	if !errors.As(err, &reqErr) {
		t.Fatalf("err = %v, want a *dflhttp.ReqError", err)
	}

	if reqErr.StatusCode != http.StatusInternalServerError || reqErr.Code != "version_unconfigured" {
		t.Errorf("got %d %s, want 500 version_unconfigured", reqErr.StatusCode, reqErr.Code)
	}
}

// TestVersionsReportsAscending checks introspection: Versions returns the
// registered set oldest first regardless of registration order.
func TestVersionsReportsAscending(t *testing.T) {
	api := version.NewResolver(version.Dates(), version.Header("X-API-Version"))

	e := version.NewEndpoint(api)
	version.Handle(e, dateV2, handleUserV2)
	version.Handle(e, dateV1, handleUserV1)

	got := e.Versions()
	if len(got) != 2 {
		t.Fatalf("versions = %v, want 2 entries", got)
	}

	if got[0].Format(time.DateOnly) != dateV1 || got[1].Format(time.DateOnly) != dateV2 {
		t.Errorf("versions = %v, want ascending [%s %s]", got, dateV1, dateV2)
	}
}

// TestRegistrationPanics pins the fail-fast contract shared with
// Router.Handle: bad versions, duplicates, and unbindable Reqs all panic
// at registration.
func TestRegistrationPanics(t *testing.T) {
	newEndpoint := func() *version.Endpoint[time.Time] {
		return version.NewEndpoint(version.NewResolver(version.Dates(), version.Header("X-API-Version")))
	}

	t.Run("unparseable version", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected Handle to panic on a version the scheme rejects")
			}
		}()

		version.Handle(newEndpoint(), "banana", handleUserV1)
	})

	t.Run("duplicate version", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected Handle to panic on a duplicate version")
			}
		}()

		e := newEndpoint()
		version.Handle(e, dateV1, handleUserV1)
		version.Handle(e, dateV1, handleUserV2)
	})

	t.Run("unbindable req", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected Handle to panic on a Req the parser can't bind")
			}
		}()

		type badReq struct {
			C chan int `query:"c"`
		}

		version.Handle(newEndpoint(), dateV1, func(context.Context, *badReq) (*dflhttp.Empty, error) {
			return &dflhttp.Empty{}, nil
		})
	})
}
