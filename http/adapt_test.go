package http_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dflhttp "github.com/duffleone/dfl/http"
)

type echoReq struct {
	Name string `query:"name"`
}

type echoResp struct {
	Name string `json:"name"`
}

// TestAdaptParsesCallsAndEncodes checks the exported Adapt does the full
// typed round trip on its own, with no Router involved: bind Req, call the
// handler, JSON-encode Resp.
func TestAdaptParsesCallsAndEncodes(t *testing.T) {
	h, err := dflhttp.Adapt(nil, func(_ context.Context, req *echoReq) (*echoResp, error) {
		return &echoResp{Name: req.Name}, nil
	})
	if err != nil {
		t.Fatalf("adapt: %v", err)
	}

	rec := httptest.NewRecorder()
	if err := h(rec, httptest.NewRequest(http.MethodGet, "/?name=ada", nil)); err != nil {
		t.Fatalf("serve: %v", err)
	}

	if got, want := strings.TrimSpace(rec.Body.String()), `{"name":"ada"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content type = %q, want application/json", got)
	}
}

// TestAdaptEmptyRespWrites204 pins the Empty contract outside the Router:
// an *Empty response is a 204 with no body.
func TestAdaptEmptyRespWrites204(t *testing.T) {
	h, err := dflhttp.Adapt(nil, func(context.Context, *dflhttp.Empty) (*dflhttp.Empty, error) {
		return &dflhttp.Empty{}, nil
	})
	if err != nil {
		t.Fatalf("adapt: %v", err)
	}

	rec := httptest.NewRecorder()
	if err := h(rec, httptest.NewRequest(http.MethodDelete, "/", nil)); err != nil {
		t.Fatalf("serve: %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
}

// TestAdaptReturnsHandlerError checks errors surface to the caller intact
// and unwritten, so Adapt output composes with any error pipeline.
func TestAdaptReturnsHandlerError(t *testing.T) {
	sentinel := errors.New("boom")

	h, err := dflhttp.Adapt(nil, func(context.Context, *dflhttp.Empty) (*dflhttp.Empty, error) {
		return nil, sentinel
	})
	if err != nil {
		t.Fatalf("adapt: %v", err)
	}

	rec := httptest.NewRecorder()

	err = h(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the handler's own error", err)
	}

	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want nothing written on error", rec.Body.String())
	}
}

// TestAdaptRejectsUnbindableReq checks shape verification happens at Adapt
// time: a Req the parser can't bind is an error up front, not a request-time
// failure.
func TestAdaptRejectsUnbindableReq(t *testing.T) {
	type badReq struct {
		C chan int `query:"c"`
	}

	_, err := dflhttp.Adapt(nil, func(context.Context, *badReq) (*dflhttp.Empty, error) {
		return &dflhttp.Empty{}, nil
	})
	if err == nil {
		t.Fatal("adapt: want an error for the unbindable Req, got nil")
	}
}
