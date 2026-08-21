package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dflhttp "github.com/duffleone/dfl/http"
)

// serve runs one request through a fresh std-mux router configured by
// register, returning the recorder.
func serve(t *testing.T, register func(r *dflhttp.Router), method, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	r := dflhttp.NewRouter(mux)

	register(r)

	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

// TestRecovererConvertsPanicTo500: a panicking handler becomes a 500
// "unknown" response instead of a dead connection, and the panic value
// stays off the wire.
func TestRecovererConvertsPanicTo500(t *testing.T) {
	rec := serve(t, func(r *dflhttp.Router) {
		r.Use(dflhttp.Recoverer())
		r.Handle(http.MethodGet, "/boom",
			func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
				panic("secret internal state")
			})
	}, http.MethodGet, "/boom", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	var body dflhttp.ReqError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != "unknown" {
		t.Errorf("body = %q, want an unknown-coded ReqError", rec.Body.String())
	}

	if strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("body = %q, the panic value must not reach the wire", rec.Body.String())
	}
}

// TestRecovererRepanicsAbortHandler: http.ErrAbortHandler is net/http's
// sanctioned way to abort a response, and must pass through untouched.
func TestRecovererRepanicsAbortHandler(t *testing.T) {
	h := dflhttp.Recoverer()(func(http.ResponseWriter, *http.Request) error {
		panic(http.ErrAbortHandler)
	})

	defer func() {
		if p := recover(); p != http.ErrAbortHandler { //nolint:errorlint // identity is the contract
			t.Errorf("recovered %v, want http.ErrAbortHandler to re-panic", p)
		}
	}()

	_ = h(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

// TestRequestMeta covers both trust modes: X-Forwarded-For wins only when
// trustForwarded is set, and User-Agent rides along either way.
func TestRequestMeta(t *testing.T) {
	cases := []struct {
		name   string
		trust  bool
		wantIP string
	}{
		{"forwarded ignored by default", false, "192.0.2.1"},
		{"forwarded trusted when asked", true, "203.0.113.9"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotIP, gotUA string

			rec := serve(t, func(r *dflhttp.Router) {
				r.Use(dflhttp.RequestMeta(tc.trust))
				r.Handle(http.MethodGet, "/x",
					func(ctx context.Context, _ *dflhttp.Empty) (*dflhttp.Empty, error) {
						gotIP, gotUA = dflhttp.ClientIP(ctx), dflhttp.UserAgent(ctx)

						return nil, nil //nolint:nilnil // 204 shape
					})
			}, http.MethodGet, "/x", map[string]string{
				"X-Forwarded-For": "203.0.113.9, 10.0.0.1",
				"User-Agent":      "dfl-test/1",
			})

			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", rec.Code)
			}

			// httptest.NewRequest sets RemoteAddr to 192.0.2.1:1234.
			if gotIP != tc.wantIP {
				t.Errorf("ClientIP = %q, want %q", gotIP, tc.wantIP)
			}

			if gotUA != "dfl-test/1" {
				t.Errorf("UserAgent = %q, want dfl-test/1", gotUA)
			}
		})
	}
}

// TestMetaAccessorsOutsideMiddleware: without RequestMeta in the chain the
// accessors return "", not garbage and not a panic.
func TestMetaAccessorsOutsideMiddleware(t *testing.T) {
	if got := dflhttp.ClientIP(t.Context()); got != "" {
		t.Errorf("ClientIP = %q, want empty", got)
	}

	if got := dflhttp.UserAgent(t.Context()); got != "" {
		t.Errorf("UserAgent = %q, want empty", got)
	}
}

// TestNotFoundHandlers: the two mux-fallback handlers emit dfl's error
// shape on the statuses their codes derive.
func TestNotFoundHandlers(t *testing.T) {
	cases := []struct {
		name       string
		handler    http.HandlerFunc
		wantCode   string
		wantStatus int
	}{
		{"route not found", dflhttp.NotFoundHandler(), "route_not_found", http.StatusNotFound},
		{"method not allowed", dflhttp.MethodNotAllowedHandler(), "method_not_allowed", http.StatusMethodNotAllowed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.handler(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			var body dflhttp.ReqError
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != tc.wantCode {
				t.Errorf("body = %q, want code %q", rec.Body.String(), tc.wantCode)
			}
		})
	}
}

// errGoneConflict is the domain-sentinel pattern: a package-level ReqError
// carrying code and status at the point of definition. The test pins that
// DefaultCoercer resolves it through any wrapping, which is what keeps the
// coercer from growing a case per domain.
var errGoneConflict = dflhttp.New("holding_conflict", nil).WithStatus(http.StatusConflict)

func TestSentinelReqErrorSurvivesWrapping(t *testing.T) {
	rec := serve(t, func(r *dflhttp.Router) {
		r.Handle(http.MethodPost, "/holdings",
			func(_ context.Context, _ *dflhttp.Empty) (*string, error) {
				return nil, fmt.Errorf("create holding: %w", errGoneConflict)
			})
	}, http.MethodPost, "/holdings", nil)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 from the sentinel's WithStatus", rec.Code)
	}

	var body dflhttp.ReqError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != "holding_conflict" {
		t.Errorf("body = %q, want the sentinel's code", rec.Body.String())
	}

	if !errors.Is(fmt.Errorf("x: %w", errGoneConflict), errGoneConflict) {
		t.Error("errors.Is must keep matching the sentinel through wrapping")
	}
}
