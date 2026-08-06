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

	e := dflhttp.New("bad_request", dflhttp.M{"x": 1}, cause, other)

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

	e := dflhttp.Wrap(primary, "boom", nil, secondary)

	if !slices.Equal(e.Unwrap(), []error{primary, secondary}) {
		t.Errorf("Unwrap() = %v, want [primary, secondary]", e.Unwrap())
	}

	if !errors.Is(e, primary) {
		t.Errorf("errors.Is(reqErr, primary) should be true")
	}
}

// TestStatusCodeFromCode walks the code-to-status table, including the 400
// default for a code nothing has claimed. 400 is the default on purpose: a
// ReqError is an error somebody wrote down, so it's part of the contract
// rather than something that should page anyone.
func TestStatusCodeFromCode(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{"bad_request", http.StatusBadRequest},
		{"unauthorized", http.StatusUnauthorized},
		{"access_denied", http.StatusForbidden},
		{"not_found", http.StatusNotFound},
		{"route_not_found", http.StatusNotFound},
		{"method_not_allowed", http.StatusMethodNotAllowed},
		{"endpoint_withdrawn", http.StatusGone},
		{"unsupported_media_type", http.StatusUnsupportedMediaType},
		{"too_many_requests", http.StatusTooManyRequests},
		{"unknown", http.StatusInternalServerError},
		{"anything_service_specific", http.StatusBadRequest},
		{"", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			if got := dflhttp.New(tc.code, nil).StatusCode(); got != tc.want {
				t.Errorf("StatusCode() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestWithStatus covers the escape hatch: the statuses outside the canonical
// set have to come from somewhere, and WithStatus wins over whatever the code
// would otherwise derive.
func TestWithStatus(t *testing.T) {
	t.Run("overrides the derived status", func(t *testing.T) {
		e := dflhttp.New("name_taken", nil).WithStatus(http.StatusConflict)

		if got := e.StatusCode(); got != http.StatusConflict {
			t.Errorf("StatusCode() = %d, want 409", got)
		}
	})

	t.Run("overrides a code the table knows", func(t *testing.T) {
		e := dflhttp.New("not_found", nil).WithStatus(http.StatusGone)

		if got := e.StatusCode(); got != http.StatusGone {
			t.Errorf("StatusCode() = %d, want 410", got)
		}
	})

	t.Run("copies rather than mutating", func(t *testing.T) {
		sentinel := dflhttp.New("not_found", nil)

		if got := sentinel.WithStatus(http.StatusGone); got == sentinel {
			t.Fatalf("WithStatus should return a copy")
		}

		if got := sentinel.StatusCode(); got != http.StatusNotFound {
			t.Errorf("receiver StatusCode() = %d after WithStatus, want 404", got)
		}
	})

	t.Run("leaves the body alone", func(t *testing.T) {
		body, err := json.Marshal(dflhttp.New("name_taken", nil).WithStatus(http.StatusConflict))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		if want := `{"code":"name_taken"}`; string(body) != want {
			t.Errorf("wire shape = %s, want %s", body, want)
		}
	})
}

// TestStatusIsNotOnTheWire pins the shape: the status travels on the status
// line, and the body is cher's {code, meta, reasons} with nothing extra.
func TestStatusIsNotOnTheWire(t *testing.T) {
	e := dflhttp.New("not_found", dflhttp.M{"id": "42"}).WithStatus(http.StatusGone)

	body, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(body), "status") {
		t.Errorf("body should carry no status, got %s", body)
	}

	if want := `{"code":"not_found","meta":{"id":"42"}}`; string(body) != want {
		t.Errorf("wire shape = %s, want %s", body, want)
	}
}

// TestReqErrorUnwrap covers the multi-error form: nil with no causes,
// every cause otherwise, and errors.Is traversal through any branch, not
// just the first.
func TestReqErrorUnwrap(t *testing.T) {
	t.Run("returns nil when no causes", func(t *testing.T) {
		e := dflhttp.New("x", nil)

		if got := e.Unwrap(); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
	})

	t.Run("returns every cause", func(t *testing.T) {
		first := errors.New("first")
		second := errors.New("second")

		e := dflhttp.New("x", nil, first, second)

		if got := e.Unwrap(); !slices.Equal(got, []error{first, second}) {
			t.Errorf("Unwrap() = %v, want [first, second]", got)
		}
	})

	t.Run("errors.Is walks transitively through any cause", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		inner := fmt.Errorf("layer: %w", sentinel)
		unrelated := errors.New("unrelated")

		e := dflhttp.New("x", nil, unrelated, inner)

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
	sentinel := dflhttp.New("invalid_team", nil)

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

	want := `{"code":"invalid_team",` +
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

// TestReqErrorNestedReasons: a reason carries its own reasons, so a check
// that decomposes keeps its shape on the wire instead of flattening into one
// list where the client has to guess which failure belongs to which field.
func TestReqErrorNestedReasons(t *testing.T) {
	e := dflhttp.New("validation_failed", nil).WithReasons(
		dflhttp.Reason{
			Code: "invalid",
			Meta: dflhttp.M{"field": "address"},
			Reasons: []dflhttp.Reason{
				{Code: "required", Meta: dflhttp.M{"field": "postcode"}},
				{
					Code: "invalid",
					Meta: dflhttp.M{"field": "country"},
					Reasons: []dflhttp.Reason{
						{Code: "not_supported"},
					},
				},
			},
		},
	)

	body, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"code":"validation_failed","reasons":[` +
		`{"code":"invalid","meta":{"field":"address"},"reasons":[` +
		`{"code":"required","meta":{"field":"postcode"}},` +
		`{"code":"invalid","meta":{"field":"country"},"reasons":[{"code":"not_supported"}]}` +
		`]}]}`
	if string(body) != want {
		t.Errorf("wire shape = %s,\nwant %s", body, want)
	}
}

// TestReqErrorError checks the Error() string format. Not a stable contract
// for callers to parse, but worth pinning down so accidental changes are
// caught.
func TestReqErrorError(t *testing.T) {
	e := dflhttp.New("bad_request", dflhttp.M{"x": 1, "y": 2})

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
// nil and *ReqError, otherwise wrap as "unknown", which is one of the codes
// that does mean 500. An error nothing classified is a bug, not a contract.
func TestDefaultCoercer(t *testing.T) {
	t.Run("nil in, nil out", func(t *testing.T) {
		if got := dflhttp.DefaultCoercer(nil); got != nil {
			t.Errorf("DefaultCoercer(nil) = %v, want nil", got)
		}
	})

	t.Run("returns *ReqError unchanged", func(t *testing.T) {
		in := dflhttp.New("missing", nil)

		if got := dflhttp.DefaultCoercer(in); got != in {
			t.Errorf("DefaultCoercer should return the same instance")
		}
	})

	t.Run("unwraps wrapped *ReqError via errors.As", func(t *testing.T) {
		in := dflhttp.New("upstream", nil)
		wrapped := fmt.Errorf("layer: %w", in)

		if got := dflhttp.DefaultCoercer(wrapped); got != in {
			t.Errorf("DefaultCoercer should unwrap to the inner *ReqError")
		}
	})

	t.Run("unknown error becomes 500 unknown wrapping the original", func(t *testing.T) {
		original := errors.New("kaboom")

		out := dflhttp.DefaultCoercer(original)

		if out.StatusCode() != http.StatusInternalServerError {
			t.Errorf("StatusCode() = %d, want 500", out.StatusCode())
		}

		if out.Code != "unknown" {
			t.Errorf("Code = %q, want %q", out.Code, "unknown")
		}

		if !errors.Is(out, original) {
			t.Errorf("coerced error should wrap the original via errors.Is")
		}
	})
}
