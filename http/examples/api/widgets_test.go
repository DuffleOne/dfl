package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duffleone/dfl/http/examples/api"
	dflhttp "github.com/duffleone/dfl/http"
)

// TestCreateWidgetValidationFailure exercises the validation path: a single
// request fails validation in all three input sources (path, query, body)
// at once. The response should be a 400 with code "validation_failed" and
// a meta.fields object that names every offending field.
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
		Code       string         `json:"code"`
		StatusCode int            `json:"status_code"`
		Meta       map[string]any `json:"meta"`
	}

	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode body: %v body=%s", err, raw)
	}

	if got.Code != "validation_failed" {
		t.Errorf("code = %q, want validation_failed", got.Code)
	}

	if got.StatusCode != http.StatusBadRequest {
		t.Errorf("body statusCode = %d, want 400", got.StatusCode)
	}

	fields, ok := got.Meta["fields"].(map[string]any)
	if !ok {
		t.Fatalf("meta.fields missing or wrong shape: %v", got.Meta)
	}

	wantFields := map[string]string{
		"id":    "must be a positive integer",
		"qty":   "must be between 1 and 100",
		"name":  "is required",
		"color": `must be one of red, blue, green (got "purple")`,
	}

	for name, wantMsg := range wantFields {
		gotMsg, present := fields[name]
		if !present {
			t.Errorf("field %q missing from response: %v", name, fields)

			continue
		}

		if gotMsg != wantMsg {
			t.Errorf("field %q: got %q, want %q", name, gotMsg, wantMsg)
		}
	}

	if len(fields) != len(wantFields) {
		t.Errorf("got %d field errors (%v), want %d (%v)", len(fields), fields, len(wantFields), wantFields)
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
