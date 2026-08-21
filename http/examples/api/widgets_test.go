package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dflhttp "github.com/duffleone/dfl/http"
	"github.com/duffleone/dfl/http/examples/api"
)

// TestCreateWidgetValidationFailure exercises the validation path: a single
// request fails validation in all three input sources (path, query, body)
// at once. The response should be a 400 with code "validation_failed" and
// a reasons array naming every offending field, {in, field} per entry,
// the same shape binding failures use.
func TestCreateWidgetValidationFailure(t *testing.T) {
	r := dflhttp.NewRouter(http.NewServeMux())
	api.NewWidgets().Mount(r)

	srv := httptest.NewServer(r)
	defer srv.Close()

	body := strings.NewReader(`{"name":"","color":"purple"}`)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/widgets/0?qty=999", body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var got struct {
		Code    string `json:"code"`
		Reasons []struct {
			Code string         `json:"code"`
			Meta map[string]any `json:"meta"`
		} `json:"reasons"`
	}

	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode body: %v body=%s", err, raw)
	}

	if got.Code != "validation_failed" {
		t.Errorf("code = %q, want validation_failed", got.Code)
	}

	byField := map[string]struct{ code, in string }{}

	for _, reason := range got.Reasons {
		field, _ := reason.Meta["field"].(string)
		in, _ := reason.Meta["in"].(string)
		byField[field] = struct{ code, in string }{reason.Code, in}
	}

	want := map[string]struct{ code, in string }{
		"id":    {"invalid", "path"},
		"qty":   {"invalid", "query"},
		"name":  {"required", "body"},
		"color": {"invalid", "body"},
	}

	for field, w := range want {
		g, present := byField[field]
		if !present {
			t.Errorf("field %q missing from reasons: %v", field, got.Reasons)

			continue
		}

		if g != w {
			t.Errorf("field %q: got %+v, want %+v", field, g, w)
		}
	}

	if len(got.Reasons) != len(want) {
		t.Errorf("got %d reasons (%v), want %d", len(got.Reasons), got.Reasons, len(want))
	}
}

// TestCreateWidgetSuccess covers the happy path: valid input passes
// validation, the widget is created, and the response echoes it.
func TestCreateWidgetSuccess(t *testing.T) {
	r := dflhttp.NewRouter(http.NewServeMux())
	api.NewWidgets().Mount(r)

	srv := httptest.NewServer(r)
	defer srv.Close()

	body := strings.NewReader(`{"name":"sprocket","color":"red"}`)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/widgets/42?qty=5", body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200, body=%s", resp.StatusCode, raw)
	}

	var got api.Widget
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	want := api.Widget{ID: 42, Qty: 5, Name: "sprocket", Color: "red"}
	if got != want {
		t.Errorf("widget = %+v, want %+v", got, want)
	}
}
