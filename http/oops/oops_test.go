package oops

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	dflhttp "github.com/duffleone/dfl/http"
	samberoops "github.com/samber/oops"
)

// TestCodeFromMessage covers snake_case derivation: trim, lowercase, replace
// spaces with underscores, strip non-[a-z0-9_] characters, collapse runs of
// underscores, trim leading/trailing underscores. Note that punctuation is
// stripped, not converted to underscore - so "only-junk" becomes "onlyjunk",
// not "only_junk".
func TestCodeFromMessage(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"hello", "hello"},
		{"Hello World", "hello_world"},
		{"  spaces  trim  ", "spaces_trim"},
		{"already_snake", "already_snake"},
		{"only-junk", "onlyjunk"},
		{"!!only-junk!!", "onlyjunk"},
		{"multiple   spaces", "multiple_spaces"},
		{"!!", ""},
		{"_leading_and_trailing_", "leading_and_trailing"},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := codeFromMessage(tc.in); got != tc.want {
				t.Errorf("codeFromMessage(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestNormaliseOopsCode is defensive against samber/oops API drift: the
// helper accepts any, returns trimmed string, treats nil and empty as
// missing.
func TestNormaliseOopsCode(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil interface returns empty", nil, ""},
		{"empty string returns empty", "", ""},
		{"whitespace-only returns empty", "  \t  ", ""},
		{"plain string passes through", "user_not_found", "user_not_found"},
		{"trims surrounding whitespace", "  user_not_found  ", "user_not_found"},
		{"non-string is fmt-formatted", 42, "42"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normaliseOopsCode(tc.in); got != tc.want {
				t.Errorf("normaliseOopsCode(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCoerceNil checks the trivial case.
func TestCoerceNil(t *testing.T) {
	if got := Coerce(nil); got != nil {
		t.Errorf("Coerce(nil) = %v, want nil", got)
	}
}

// TestCoercePassesThroughReqError: a *ReqError on the way in is returned
// unchanged (same instance), so that handlers using dflhttp.New keep their
// status, code, and meta intact.
func TestCoercePassesThroughReqError(t *testing.T) {
	in := dflhttp.New(http.StatusNotFound, "missing", dflhttp.M{"id": "x"})

	if got := Coerce(in); got != in {
		t.Errorf("Coerce should return the same *ReqError instance")
	}
}

// TestCoerceUnwrapsWrappedReqError: errors.As lets us find a *ReqError even
// when wrapped by fmt.Errorf or anything else.
func TestCoerceUnwrapsWrappedReqError(t *testing.T) {
	in := dflhttp.New(http.StatusBadGateway, "upstream", nil)
	wrapped := fmt.Errorf("layer: %w", in)

	if got := Coerce(wrapped); got != in {
		t.Errorf("Coerce should unwrap to the inner *ReqError")
	}
}

// TestCoerceOopsWithCode: a samber/oops error with Code, Context, and Public
// projects all three onto the ReqError. Status is always 500 since oops
// errors are server-side.
func TestCoerceOopsWithCode(t *testing.T) {
	err := samberoops.
		Code("user_not_found").
		With("user_id", "abc").
		Public("not found").
		Errorf("user %s not found", "abc")

	out := Coerce(err)

	if out.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", out.StatusCode)
	}

	if out.Code != "user_not_found" {
		t.Errorf("Code = %q, want %q", out.Code, "user_not_found")
	}

	if got, _ := out.Meta["user_id"].(string); got != "abc" {
		t.Errorf("Meta[user_id] = %v, want %q", out.Meta["user_id"], "abc")
	}

	if got, _ := out.Meta["public"].(string); got != "not found" {
		t.Errorf("Meta[public] = %v, want %q", out.Meta["public"], "not found")
	}
}

// TestCoerceOopsWithoutCode: when the oops error has no explicit code, the
// coercer derives one from the inner error's message.
func TestCoerceOopsWithoutCode(t *testing.T) {
	inner := errors.New("Widget Exploded")
	err := samberoops.Wrap(inner)

	out := Coerce(err)

	if out.Code != "widget_exploded" {
		t.Errorf("Code = %q, want %q", out.Code, "widget_exploded")
	}
}

// TestCoerceGenericWrappedError: a non-oops error that has an Unwrap chain
// gets a code derived from the unwrapped message.
func TestCoerceGenericWrappedError(t *testing.T) {
	inner := errors.New("Database Connection Lost")
	err := fmt.Errorf("query failed: %w", inner)

	out := Coerce(err)

	if out.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", out.StatusCode)
	}

	if out.Code != "database_connection_lost" {
		t.Errorf("Code = %q, want %q", out.Code, "database_connection_lost")
	}
}

// TestCoercePlainError: an error with no chain falls all the way through to
// the "unknown" 500 default.
func TestCoercePlainError(t *testing.T) {
	err := errors.New("boom")

	out := Coerce(err)

	if out.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", out.StatusCode)
	}

	if out.Code != "unknown" {
		t.Errorf("Code = %q, want %q", out.Code, "unknown")
	}
}
