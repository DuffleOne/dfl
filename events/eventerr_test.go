package events_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/duffleone/dfl/events"
)

func TestMKeys(t *testing.T) {
	cases := []struct {
		name string
		m    events.M
		want []string
	}{
		{"nil map yields empty slice", nil, []string{}},
		{"empty map yields empty slice", events.M{}, []string{}},
		{"single key", events.M{"a": 1}, []string{"a"}},
		{"multiple keys returned in any order", events.M{"a": 1, "b": 2, "c": 3}, []string{"a", "b", "c"}},
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

func TestEventErrorNew(t *testing.T) {
	cause := errors.New("cause")

	e := events.New("bad", events.M{"x": 1}, cause)

	if e.Code != "bad" {
		t.Errorf("Code = %q, want bad", e.Code)
	}

	if got := e.Meta["x"]; got != 1 {
		t.Errorf("Meta[x] = %v, want 1", got)
	}

	if !slices.Equal(e.Reasons, []error{cause}) {
		t.Errorf("Reasons = %v, want [cause]", e.Reasons)
	}
}

func TestEventErrorWrap(t *testing.T) {
	primary := errors.New("primary")
	secondary := errors.New("secondary")

	e := events.Wrap(primary, "boom", nil, secondary)

	if len(e.Reasons) != 2 {
		t.Fatalf("Reasons len = %d, want 2", len(e.Reasons))
	}

	if e.Reasons[0] != primary {
		t.Errorf("Reasons[0] = %v, want primary", e.Reasons[0])
	}

	if !errors.Is(e, primary) {
		t.Errorf("errors.Is(eventErr, primary) should be true")
	}
}

func TestEventErrorUnwrap(t *testing.T) {
	t.Run("nil when no reasons", func(t *testing.T) {
		if got := events.New("x", nil).Unwrap(); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
	})

	t.Run("first reason otherwise", func(t *testing.T) {
		first := errors.New("first")

		if got := events.New("x", nil, first, errors.New("second")).Unwrap(); got != first {
			t.Errorf("Unwrap() = %v, want first", got)
		}
	})

	t.Run("errors.Is walks transitively", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		inner := fmt.Errorf("layer: %w", sentinel)

		if !errors.Is(events.New("x", nil, inner), sentinel) {
			t.Errorf("errors.Is should reach the sentinel through Reasons[0]")
		}
	})
}

func TestEventErrorError(t *testing.T) {
	got := events.New("bad", events.M{"x": 1, "y": 2}).Error()

	if !strings.HasPrefix(got, "bad keys=") {
		t.Errorf("Error() = %q, want prefix %q", got, "bad keys=")
	}

	for _, k := range []string{"x", "y"} {
		if !strings.Contains(got, k) {
			t.Errorf("Error() = %q, expected to contain key %q", got, k)
		}
	}
}

func TestDefaultCoercer(t *testing.T) {
	t.Run("nil in, nil out", func(t *testing.T) {
		if got := events.DefaultCoercer(nil); got != nil {
			t.Errorf("DefaultCoercer(nil) = %v, want nil", got)
		}
	})

	t.Run("returns *EventError unchanged", func(t *testing.T) {
		in := events.New("missing", nil)

		if got := events.DefaultCoercer(in); got != in {
			t.Errorf("DefaultCoercer should return the same instance")
		}
	})

	t.Run("unwraps wrapped *EventError via errors.As", func(t *testing.T) {
		in := events.New("upstream", nil)
		wrapped := fmt.Errorf("layer: %w", in)

		if got := events.DefaultCoercer(wrapped); got != in {
			t.Errorf("DefaultCoercer should unwrap to the inner *EventError")
		}
	})

	t.Run("unknown error becomes code unknown wrapping the original", func(t *testing.T) {
		original := errors.New("kaboom")

		out := events.DefaultCoercer(original)

		if out.Code != "unknown" {
			t.Errorf("Code = %q, want unknown", out.Code)
		}

		if !errors.Is(out, original) {
			t.Errorf("coerced error should wrap the original via errors.Is")
		}
	})
}
