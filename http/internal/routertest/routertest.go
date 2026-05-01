// Package routertest exports a conformance suite that every dflhttp.Router
// backend must pass. Each backend's *_test.go calls Run with a Factory that
// builds an instance of that backend; the suite then exercises the
// observable behaviour we care about (binding, errors, groups, middleware).
//
// Tests live here and not in each backend so both std and chi (and any
// future backend) are verified by literally the same code.
package routertest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	dflhttp "github.com/duffleone/dfl/http"
)

// Factory tells the suite how to build a fresh router for each test. Each
// backend supplies one of these in its *_test.go.
type Factory struct {
	// New returns a default-configured router and the same instance as an
	// http.Handler for ServeHTTP dispatch.
	New func() (*dflhttp.Router, http.Handler)

	// NewWithCoercer returns a router configured with the given Coercer.
	// Used by the WithCoercer test only.
	NewWithCoercer func(dflhttp.Coercer) (*dflhttp.Router, http.Handler)
}

// Run executes the full conformance suite against f. Call it from a single
// top-level test in each backend's package.
func Run(t *testing.T, f Factory) {
	t.Helper()

	t.Run("StringRespIsJSONEncoded", func(t *testing.T) { stringRespIsJSONEncoded(t, f) })
	t.Run("EmptyRespReturns204", func(t *testing.T) { emptyRespReturns204(t, f) })
	t.Run("EmptyPointerRespReturns204", func(t *testing.T) { emptyPointerRespReturns204(t, f) })
	t.Run("PathBinding", func(t *testing.T) { pathBinding(t, f) })
	t.Run("PathBindingInvalid", func(t *testing.T) { pathBindingInvalid(t, f) })
	t.Run("QueryBinding", func(t *testing.T) { queryBinding(t, f) })
	t.Run("QueryBindingMissingLeavesZero", func(t *testing.T) { queryBindingMissingLeavesZero(t, f) })
	t.Run("QueryBindingInvalid", func(t *testing.T) { queryBindingInvalid(t, f) })
	t.Run("BodyBinding", func(t *testing.T) { bodyBinding(t, f) })
	t.Run("BodyBindingRejectsNonJSON", func(t *testing.T) { bodyBindingRejectsNonJSON(t, f) })
	t.Run("BodyBindingMalformedJSON", func(t *testing.T) { bodyBindingMalformedJSON(t, f) })
	t.Run("PathPlusBody", func(t *testing.T) { pathPlusBody(t, f) })
	t.Run("BodyDoesntBleedToNonJSONFields", func(t *testing.T) { bodyDoesntBleedToNonJSONFields(t, f) })
	t.Run("ReqErrorPropagates", func(t *testing.T) { reqErrorPropagates(t, f) })
	t.Run("GenericErrorBecomes500", func(t *testing.T) { genericErrorBecomes500(t, f) })
	t.Run("GroupPrefixesPaths", func(t *testing.T) { groupPrefixesPaths(t, f) })
	t.Run("MiddlewareWraps", func(t *testing.T) { middlewareWraps(t, f) })
	t.Run("MiddlewareShortCircuit", func(t *testing.T) { middlewareShortCircuit(t, f) })
	t.Run("UseAfterGroupDoesNotPropagate", func(t *testing.T) { useAfterGroupDoesNotPropagate(t, f) })
	t.Run("PerRouteMiddleware", func(t *testing.T) { perRouteMiddleware(t, f) })
	t.Run("WithCoercer", func(t *testing.T) { withCoercer(t, f) })
	t.Run("PanicOnInvalidReq", func(t *testing.T) { panicOnInvalidReq(t, f) })
}

// do dispatches a request through h and returns the recorder. Tests inspect
// status, body, and headers on the result.
func do(h http.Handler, method, target string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, body)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func jsonHeaders() map[string]string {
	return map[string]string{"Content-Type": "application/json"}
}

// stringRespIsJSONEncoded: a primitive Resp like string still goes through
// json.Encode, producing a quoted JSON string and the application/json
// content type.
func stringRespIsJSONEncoded(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	dflhttp.Handle(r, http.MethodGet, "/health",
		func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
			return new("up"), nil
		})

	rec := do(h, http.MethodGet, "/health", nil, nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	if got := strings.TrimSpace(rec.Body.String()); got != `"up"` {
		t.Errorf("body = %q, want %q", got, `"up"`)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// emptyRespReturns204: the Empty sentinel produces 204, no body, no
// Content-Type.
func emptyRespReturns204(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	dflhttp.Handle(r, http.MethodPost, "/ping",
		func(_ context.Context, _ *dflhttp.Empty) (*dflhttp.Empty, error) {
			return &dflhttp.Empty{}, nil
		})

	rec := do(h, http.MethodPost, "/ping", nil, nil)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

// emptyPointerRespReturns204: returning (*Empty, nil) is treated the same
// as (Empty, nil). nil pointer is the natural shape for handlers that only
// signal "done".
func emptyPointerRespReturns204(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	dflhttp.Handle(r, http.MethodPost, "/ping",
		func(_ context.Context, _ *dflhttp.Empty) (*dflhttp.Empty, error) {
			return nil, nil //nolint:nilnil // tests the (*Empty, nil) shape used by examples/health.go
		})

	rec := do(h, http.MethodPost, "/ping", nil, nil)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

type pathReq struct {
	ID    string `path:"id"`
	Count int    `path:"count"`
}

// pathBinding: typed path binding with a string and int field both extracted
// from the URL.
func pathBinding(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	var captured pathReq

	dflhttp.Handle(r, http.MethodGet, "/items/{id}/n/{count}",
		func(_ context.Context, req *pathReq) (*string, error) {
			captured = *req

			return new("ok"), nil
		})

	rec := do(h, http.MethodGet, "/items/abc/n/42", nil, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	if captured.ID != "abc" || captured.Count != 42 {
		t.Errorf("got %+v, want {ID: abc, Count: 42}", captured)
	}
}

type intPathReq struct {
	N int `path:"n"`
}

// pathBindingInvalid: a non-numeric value for an int path field is a 400
// with the standard "invalid_path_param" code.
func pathBindingInvalid(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	dflhttp.Handle(r, http.MethodGet, "/n/{n}",
		func(_ context.Context, _ *intPathReq) (*string, error) {
			return new("ok"), nil
		})

	rec := do(h, http.MethodGet, "/n/notanumber", nil, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var body dflhttp.ReqError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Code != "invalid_path_param" {
		t.Errorf("Code = %q, want invalid_path_param", body.Code)
	}
}

type queryReq struct {
	Limit int    `query:"limit"`
	Q     string `query:"q"`
	Open  bool   `query:"open"`
}

// queryBinding exercises string, int, and bool query fields together.
func queryBinding(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	var captured queryReq

	dflhttp.Handle(r, http.MethodGet, "/search",
		func(_ context.Context, req *queryReq) (*string, error) {
			captured = *req

			return new("ok"), nil
		})

	rec := do(h, http.MethodGet, "/search?limit=5&q=foo&open=true", nil, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	if captured.Limit != 5 || captured.Q != "foo" || !captured.Open {
		t.Errorf("got %+v, want {Limit: 5, Q: foo, Open: true}", captured)
	}
}

// queryBindingMissingLeavesZero: an absent query param leaves the field at
// its zero value with no error.
func queryBindingMissingLeavesZero(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	var captured queryReq

	dflhttp.Handle(r, http.MethodGet, "/search",
		func(_ context.Context, req *queryReq) (*string, error) {
			captured = *req

			return new("ok"), nil
		})

	rec := do(h, http.MethodGet, "/search", nil, nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	if captured.Limit != 0 || captured.Q != "" || captured.Open {
		t.Errorf("got %+v, want zero values", captured)
	}
}

// queryBindingInvalid: a non-bool value for a bool query field is 400.
func queryBindingInvalid(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	dflhttp.Handle(r, http.MethodGet, "/search",
		func(_ context.Context, _ *queryReq) (*string, error) {
			return new("ok"), nil
		})

	rec := do(h, http.MethodGet, "/search?open=maybe", nil, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var body dflhttp.ReqError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Code != "invalid_query_param" {
		t.Errorf("Code = %q, want invalid_query_param", body.Code)
	}
}

type bodyReq struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func bodyBinding(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	var captured bodyReq

	dflhttp.Handle(r, http.MethodPost, "/users",
		func(_ context.Context, req *bodyReq) (*string, error) {
			captured = *req

			return new("ok"), nil
		})

	rec := do(h, http.MethodPost, "/users", strings.NewReader(`{"name":"alice","age":30}`), jsonHeaders())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	if captured.Name != "alice" || captured.Age != 30 {
		t.Errorf("got %+v, want {alice, 30}", captured)
	}
}

// bodyBindingRejectsNonJSON: any non-JSON content type with a body-shaped
// Req fails fast with 415, before we try to decode.
func bodyBindingRejectsNonJSON(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	dflhttp.Handle(r, http.MethodPost, "/users",
		func(_ context.Context, _ *bodyReq) (*string, error) {
			return new("ok"), nil
		})

	rec := do(h, http.MethodPost, "/users", strings.NewReader("hi"),
		map[string]string{"Content-Type": "text/plain"})

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", rec.Code)
	}
}

// bodyBindingMalformedJSON: garbage in the body is a 400, not a 500.
func bodyBindingMalformedJSON(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	dflhttp.Handle(r, http.MethodPost, "/users",
		func(_ context.Context, _ *bodyReq) (*string, error) {
			return new("ok"), nil
		})

	rec := do(h, http.MethodPost, "/users", strings.NewReader("{not json"), jsonHeaders())

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

type updateReq struct {
	ID   string `path:"id"`
	Name string `json:"name"`
}

// pathPlusBody: a Req can mix path-bound and JSON-bound fields without one
// stepping on the other.
func pathPlusBody(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	var captured updateReq

	dflhttp.Handle(r, http.MethodPut, "/users/{id}",
		func(_ context.Context, req *updateReq) (*string, error) {
			captured = *req

			return new("ok"), nil
		})

	rec := do(h, http.MethodPut, "/users/42", strings.NewReader(`{"name":"alice"}`), jsonHeaders())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	if captured.ID != "42" || captured.Name != "alice" {
		t.Errorf("got %+v, want {42, alice}", captured)
	}
}

// bodyDoesntBleedToNonJSONFields is the security-relevant check: a body key
// that matches the field name of a path-tagged field must not overwrite the
// path-bound value. Path-tagged fields have no json: tag so they aren't in
// the body's source-of-truth.
func bodyDoesntBleedToNonJSONFields(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	var captured updateReq

	dflhttp.Handle(r, http.MethodPut, "/users/{id}",
		func(_ context.Context, req *updateReq) (*string, error) {
			captured = *req

			return new("ok"), nil
		})

	rec := do(h, http.MethodPut, "/users/42",
		strings.NewReader(`{"id":"hijacked","ID":"hijacked","name":"alice"}`), jsonHeaders())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	if captured.ID != "42" {
		t.Errorf("ID = %q, want %q (path value must not be overridden by body)", captured.ID, "42")
	}
}

// reqErrorPropagates: a *ReqError returned from a handler ends up on the
// wire with status, code, and meta intact.
func reqErrorPropagates(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	dflhttp.Handle(r, http.MethodGet, "/missing",
		func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
			return nil, dflhttp.New(http.StatusNotFound, "not_found", dflhttp.M{"id": "x"})
		})

	rec := do(h, http.MethodGet, "/missing", nil, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}

	var body dflhttp.ReqError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Code != "not_found" {
		t.Errorf("Code = %q, want not_found", body.Code)
	}

	if got, _ := body.Meta["id"].(string); got != "x" {
		t.Errorf("Meta[id] = %v, want x", body.Meta["id"])
	}
}

// genericErrorBecomes500: a plain error from a handler runs through the
// default coercer and becomes 500 "unknown".
func genericErrorBecomes500(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	dflhttp.Handle(r, http.MethodGet, "/boom",
		func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
			return nil, errors.New("kaboom")
		})

	rec := do(h, http.MethodGet, "/boom", nil, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	var body dflhttp.ReqError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Code != "unknown" {
		t.Errorf("Code = %q, want unknown", body.Code)
	}
}

// groupPrefixesPaths: registering on a Group prepends the prefix to the
// final pattern.
func groupPrefixesPaths(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()
	api := r.Group("/api")

	dflhttp.Handle(api, http.MethodGet, "/health",
		func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
			return new("up"), nil
		})

	rec := do(h, http.MethodGet, "/api/health", nil, nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// middlewareWraps verifies onion order: outermost middleware sees the
// request first and the response last; innermost is closest to the handler.
func middlewareWraps(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	var order []string

	track := func(name string) dflhttp.Middleware {
		return func(next dflhttp.HandlerFunc) dflhttp.HandlerFunc {
			return func(w http.ResponseWriter, req *http.Request) error {
				order = append(order, name+":pre")

				err := next(w, req)

				order = append(order, name+":post")

				return err
			}
		}
	}

	r.Use(track("outer"))
	r.Use(track("inner"))

	dflhttp.Handle(r, http.MethodGet, "/x",
		func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
			order = append(order, "handler")

			return new("ok"), nil
		})

	do(h, http.MethodGet, "/x", nil, nil)

	want := []string{"outer:pre", "inner:pre", "handler", "inner:post", "outer:post"}
	if !slices.Equal(order, want) {
		t.Errorf("middleware order = %v, want %v", order, want)
	}
}

// middlewareShortCircuit: if middleware returns an error before calling
// next, the handler must not run, and the error gets coerced to a response.
func middlewareShortCircuit(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	r.Use(func(_ dflhttp.HandlerFunc) dflhttp.HandlerFunc {
		return func(_ http.ResponseWriter, _ *http.Request) error {
			return dflhttp.New(http.StatusUnauthorized, "no_auth", nil)
		}
	})

	handlerCalled := false

	dflhttp.Handle(r, http.MethodGet, "/x",
		func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
			handlerCalled = true

			return new("ok"), nil
		})

	rec := do(h, http.MethodGet, "/x", nil, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}

	if handlerCalled {
		t.Errorf("handler should not have run after middleware short-circuit")
	}
}

// useAfterGroupDoesNotPropagate documents the snapshot semantics: Use'ing
// on the parent after a Group call doesn't add the middleware to that
// already-spawned group.
func useAfterGroupDoesNotPropagate(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()
	api := r.Group("/api")

	r.Use(func(_ dflhttp.HandlerFunc) dflhttp.HandlerFunc {
		return func(_ http.ResponseWriter, _ *http.Request) error {
			return dflhttp.New(http.StatusForbidden, "should_not_run", nil)
		}
	})

	dflhttp.Handle(api, http.MethodGet, "/x",
		func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
			return new("ok"), nil
		})

	rec := do(h, http.MethodGet, "/api/x", nil, nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (parent middleware should not have run on api group)", rec.Code)
	}
}

// perRouteMiddleware: middleware passed to Handle as a variadic runs inside
// any group middleware.
func perRouteMiddleware(t *testing.T, f Factory) {
	t.Helper()

	r, h := f.New()

	var ran bool

	mw := func(next dflhttp.HandlerFunc) dflhttp.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) error {
			ran = true

			return next(w, req)
		}
	}

	dflhttp.Handle(r, http.MethodGet, "/x",
		func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
			return new("ok"), nil
		}, mw)

	do(h, http.MethodGet, "/x", nil, nil)

	if !ran {
		t.Errorf("per-route middleware did not run")
	}
}

// withCoercer: replacing the default coercer changes what every handler
// error gets projected to. Here, all errors become 418.
func withCoercer(t *testing.T, f Factory) {
	t.Helper()

	teapot := func(err error) *dflhttp.ReqError {
		if err == nil {
			return nil
		}

		return dflhttp.New(http.StatusTeapot, "teapot", nil)
	}

	r, h := f.NewWithCoercer(teapot)

	dflhttp.Handle(r, http.MethodGet, "/x",
		func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
			return nil, errors.New("anything")
		})

	rec := do(h, http.MethodGet, "/x", nil, nil)

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rec.Code)
	}
}

// panicOnInvalidReq: a Req struct with an unsupported field type for a
// path tag panics at registration time, not request time. The panic
// originates in adapt() so it's the same on every backend.
func panicOnInvalidReq(t *testing.T, f Factory) {
	t.Helper()

	type badReq struct {
		Tags []string `path:"tags"` // slice can't be string-bound
	}

	r, _ := f.New()

	defer func() {
		if recover() == nil {
			t.Errorf("expected Handle to panic on unsupported field type")
		}
	}()

	dflhttp.Handle(r, http.MethodGet, "/x/{tags}",
		func(_ context.Context, _ *badReq) (*string, error) {
			return new("ok"), nil
		})
}
