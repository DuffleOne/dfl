package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/duffleone/dfl/events"
	dflhttp "github.com/duffleone/dfl/http"
)

// TestInProcess exercises the async On path: one emit fans out to two handlers.
func TestInProcess(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())

	var wg sync.WaitGroup
	wg.Add(2)

	var gotEmail, gotID string
	bus.On(func(_ context.Context, e UserCreated) error { gotEmail = e.Email; wg.Done(); return nil })
	bus.On(func(_ context.Context, e UserCreated) error { gotID = e.ID; wg.Done(); return nil })

	if err := bus.Emit(context.Background(), UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	wg.Wait()

	if gotEmail != "a@b.com" {
		t.Errorf("welcome email = %q, want a@b.com", gotEmail)
	}

	if gotID != "1" {
		t.Errorf("audit id = %q, want 1", gotID)
	}
}

// TestOverHTTP exercises RegisterEndpoint for a sanitised route, a URLSafeName
// route, and a validation failure.
func TestOverHTTP(t *testing.T) {
	bus := events.NewBus(events.NewMemSink())
	r := dflhttp.NewRouter(http.NewServeMux())
	handlers{}.MountHTTP(bus, r)

	srv := httptest.NewServer(r)
	defer srv.Close()

	cases := []struct {
		name string
		path string
		body string
		want int
	}{
		{"sanitised route", "/events/user.created", `{"id":"1","email":"a@b.com"}`, http.StatusNoContent},
		{"url-safe-name route", "/events/orders-shipped", `{"order_id":"7","carrier":"dhl"}`, http.StatusNoContent},
		{"validation failure", "/events/user.created", `{"id":"2","email":""}`, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := postStatus(t, srv, tc.path, tc.body); got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func postStatus(t *testing.T, srv *httptest.Server, path, body string) int {
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

	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode
}
