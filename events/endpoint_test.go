package events_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/duffleone/dfl/events"
	dflhttp "github.com/duffleone/dfl/http"
)

func TestRegisterEndpointSuccess(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())
	r := dflhttp.NewRouter(http.NewServeMux())

	// The endpoint runs the handler synchronously within the request, but the
	// handler runs on the server goroutine; the WaitGroup fences the read of got.
	var wg sync.WaitGroup
	wg.Add(1)

	var got evtUser
	bus.RegisterEndpoint(r, func(_ context.Context, e evtUser) error {
		got = e
		wg.Done()

		return nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := post(t, srv, "/events/test.user", `{"email":"a@b.com"}`)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	wg.Wait()

	if got.Email != "a@b.com" {
		t.Errorf("decoded email = %q, want a@b.com", got.Email)
	}
}

func TestRegisterEndpointValidationFailure(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())
	r := dflhttp.NewRouter(http.NewServeMux())

	bus.RegisterEndpoint(r, func(_ context.Context, _ evtUser) error { return nil })

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := post(t, srv, "/events/test.user", `{"email":""}`)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	var body struct {
		Code string `json:"code"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.Code != "validation_failed" {
		t.Errorf("code = %q, want validation_failed", body.Code)
	}
}

// TestRegisterEndpointCustomRoute covers the URLSafeNamer branch: the event's
// bus name is "order.shipped" but the route segment is "orders-shipped".
func TestRegisterEndpointCustomRoute(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())
	r := dflhttp.NewRouter(http.NewServeMux())

	bus.RegisterEndpoint(r, func(_ context.Context, _ evtOrder) error { return nil })

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := post(t, srv, "/events/orders-shipped", `{"id":"1"}`)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("custom route status = %d, want 204", resp.StatusCode)
	}
}

func post(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}

	return resp
}
