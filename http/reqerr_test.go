package http_test

import (
	"encoding/json"
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

// TestReqErrorNew checks the simple case: New stores all fields and records
// the variadic causes in the order they came in.
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

	if !slices.Equal(e.Unwrap(), []error{cause, other}) {
		t.Errorf("Unwrap() = %v, want [cause, other]", e.Unwrap())
	}
}

// TestReqErrorWrap verifies Wrap puts the wrapped error first among the
// causes, then any explicit causes after. Errors.Is should reach the
// wrapped error.
func TestReqErrorWrap(t *testing.T) {
	primary := errors.New("primary")
	secondary := errors.New("secondary")

	e := dflhttp.Wrap(primary, http.StatusInternalServerError, "boom", nil, secondary)

	if !slices.Equal(e.Unwrap(), []error{primary, secondary}) {
		t.Errorf("Unwrap() = %v, want [primary, secondary]", e.Unwrap())
	}

	if !errors.Is(e, primary) {
		t.Errorf("errors.Is(reqErr, primary) should be true")
	}
}

// TestReqErrorUnwrap covers the multi-error form: nil with no causes,
// every cause otherwise, and errors.Is traversal through any branch, not
// just the first.
func TestReqErrorUnwrap(t *testing.T) {
	t.Run("returns nil when no causes", func(t *testing.T) {
		e := dflhttp.New(http.StatusInternalServerError, "x", nil)

		if got := e.Unwrap(); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
	})

	t.Run("returns every cause", func(t *testing.T) {
		first := errors.New("first")
		second := errors.New("second")

		e := dflhttp.New(http.StatusInternalServerError, "x", nil, first, second)

		if got := e.Unwrap(); !slices.Equal(got, []error{first, second}) {
			t.Errorf("Unwrap() = %v, want [first, second]", got)
		}
	})

	t.Run("errors.Is walks transitively through any cause", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		inner := fmt.Errorf("layer: %w", sentinel)
		unrelated := errors.New("unrelated")

		e := dflhttp.New(http.StatusInternalServerError, "x", nil, unrelated, inner)

		if !errors.Is(e, sentinel) {
			t.Errorf("errors.Is(reqErr, sentinel) should be true via unrelated, inner -> sentinel")
		}
	})
}

// TestReqErrorReasons pins the wire contract for reasons: they serialise
// under "reasons" when present and disappear entirely when not, and
// WithReasons copies rather than mutates, so a shared sentinel ReqError
// can't leak reasons between requests.
func TestReqErrorReasons(t *testing.T) {
	sentinel := dflhttp.New(http.StatusUnprocessableEntity, "invalid_team", nil)

	decorated := sentinel.WithReasons(
		dflhttp.Reason{Code: "required", Meta: dflhttp.M{"in": "body", "field": "name"}},
		dflhttp.Reason{Code: "invalid"},
	)

	if len(sentinel.Reasons) != 0 {
		t.Fatalf("WithReasons mutated the receiver: %v", sentinel.Reasons)
	}

	body, err := json.Marshal(decorated)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"code":"invalid_team","status_code":422,` +
		`"reasons":[{"code":"required","meta":{"field":"name","in":"body"}},{"code":"invalid"}]}`
	if string(body) != want {
		t.Errorf("wire shape = %s, want %s", body, want)
	}

	bare, err := json.Marshal(sentinel)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(bare), "reasons") {
		t.Errorf("empty reasons should be omitted, got %s", bare)
	}
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
