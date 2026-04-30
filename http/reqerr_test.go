package http_test

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	dflhttp "github.com/duffleone/dfl/http"
)

// TestMKeys verifies M.Keys returns every key in the map. Iteration order
// is unspecified, so we sort before comparing.
func TestMKeys(t *testing.T) {
	cases := []struct {
		name string
		m    dflhttp.M
		want []string
	}{
		{"nil map yields empty slice", nil, []string{}},
		{"empty map yields empty slice", dflhttp.M{}, []string{}},
		{"single key", dflhttp.M{"a": 1}, []string{"a"}},
		{"multiple keys returned in any order", dflhttp.M{"a": 1, "b": 2, "c": 3}, []string{"a", "b", "c"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.m.Keys()
			slices.Sort(got)

			if !slices.Equal(got, tc.want) {
				t.Errorf("Keys() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestReqErrorNew checks the simple case: New stores all fields and treats
// the variadic reasons as Reasons in the same order they came in.
func TestReqErrorNew(t *testing.T) {
	cause := errors.New("cause")
	other := errors.New("other")

	e := dflhttp.New(http.StatusBadRequest, "bad_request", dflhttp.M{"x": 1}, cause, other)

	if e.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", e.StatusCode, http.StatusBadRequest)
	}

	if e.Code != "bad_request" {
		t.Errorf("Code = %q, want %q", e.Code, "bad_request")
	}

	if got := e.Meta["x"]; got != 1 {
		t.Errorf("Meta[x] = %v, want 1", got)
	}

	if !slices.Equal(e.Reasons, []error{cause, other}) {
		t.Errorf("Reasons = %v, want [cause, other]", e.Reasons)
	}
}

// TestReqErrorWrap verifies Wrap puts the wrapped error first in Reasons,
// then any explicit reasons after. Errors.Is should reach the wrapped error.
func TestReqErrorWrap(t *testing.T) {
	primary := errors.New("primary")
	secondary := errors.New("secondary")

	e := dflhttp.Wrap(primary, http.StatusInternalServerError, "boom", nil, secondary)

	if len(e.Reasons) != 2 {
		t.Fatalf("Reasons len = %d, want 2", len(e.Reasons))
	}

	if e.Reasons[0] != primary {
		t.Errorf("Reasons[0] = %v, want primary", e.Reasons[0])
	}

	if e.Reasons[1] != secondary {
		t.Errorf("Reasons[1] = %v, want secondary", e.Reasons[1])
	}

	if !errors.Is(e, primary) {
		t.Errorf("errors.Is(reqErr, primary) should be true")
	}
}

// TestReqErrorUnwrap covers single-step unwrap traversal: nil with no
// reasons, the first reason otherwise. Wrapped error chains traverse
// transitively because errors.Is follows the chain.
func TestReqErrorUnwrap(t *testing.T) {
	t.Run("returns nil when no reasons", func(t *testing.T) {
		e := dflhttp.New(http.StatusInternalServerError, "x", nil)

		if got := e.Unwrap(); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
	})

	t.Run("returns the first reason", func(t *testing.T) {
		first := errors.New("first")
		second := errors.New("second")

		e := dflhttp.New(http.StatusInternalServerError, "x", nil, first, second)

		if got := e.Unwrap(); got != first {
			t.Errorf("Unwrap() = %v, want first", got)
		}
	})

	t.Run("errors.Is walks transitively through Reasons[0]", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		inner := fmt.Errorf("layer: %w", sentinel)

		e := dflhttp.New(http.StatusInternalServerError, "x", nil, inner)

		if !errors.Is(e, sentinel) {
			t.Errorf("errors.Is(reqErr, sentinel) should be true via inner -> sentinel chain")
		}
	})
}

// TestReqErrorError checks the Error() string format. Not a stable contract
// for callers to parse, but worth pinning down so accidental changes are
// caught.
func TestReqErrorError(t *testing.T) {
	e := dflhttp.New(http.StatusBadRequest, "bad_request", dflhttp.M{"x": 1, "y": 2})

	got := e.Error()

	// Code prefix is fixed; key order isn't.
	if !strings.HasPrefix(got, "bad_request keys=") {
		t.Errorf("Error() = %q, want prefix %q", got, "bad_request keys=")
	}

	// The keys (x, y) should both be in the message.
	for _, k := range []string{"x", "y"} {
		if !strings.Contains(got, k) {
			t.Errorf("Error() = %q, expected to contain key %q", got, k)
		}
	}
}

// TestDefaultCoercer covers the minimal pluggable default: pass through
// nil and *ReqError, otherwise wrap as 500 "unknown".
func TestDefaultCoercer(t *testing.T) {
	t.Run("nil in, nil out", func(t *testing.T) {
		if got := dflhttp.DefaultCoercer(nil); got != nil {
			t.Errorf("DefaultCoercer(nil) = %v, want nil", got)
		}
	})

	t.Run("returns *ReqError unchanged", func(t *testing.T) {
		in := dflhttp.New(http.StatusNotFound, "missing", nil)

		if got := dflhttp.DefaultCoercer(in); got != in {
			t.Errorf("DefaultCoercer should return the same instance")
		}
	})

	t.Run("unwraps wrapped *ReqError via errors.As", func(t *testing.T) {
		in := dflhttp.New(http.StatusBadGateway, "upstream", nil)
		wrapped := fmt.Errorf("layer: %w", in)

		if got := dflhttp.DefaultCoercer(wrapped); got != in {
			t.Errorf("DefaultCoercer should unwrap to the inner *ReqError")
		}
	})

	t.Run("unknown error becomes 500 unknown wrapping the original", func(t *testing.T) {
		original := errors.New("kaboom")

		out := dflhttp.DefaultCoercer(original)

		if out.StatusCode != http.StatusInternalServerError {
			t.Errorf("StatusCode = %d, want 500", out.StatusCode)
		}

		if out.Code != "unknown" {
			t.Errorf("Code = %q, want %q", out.Code, "unknown")
		}

		if !errors.Is(out, original) {
			t.Errorf("coerced error should wrap the original via errors.Is")
		}
	})
}
