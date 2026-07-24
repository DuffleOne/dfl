package version_test

import (
	"strconv"
	"testing"

	"github.com/duffleone/dfl/http/version"
)

// TestDatesParsesOrdersAndFormats covers the date scheme's round trip:
// DateOnly strings in, chronological order, canonical rendering out.
func TestDatesParsesOrdersAndFormats(t *testing.T) {
	s := version.Dates()

	older, err := s.Parse(dateV1)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	newer, err := s.Parse(dateV2)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got := s.Compare(older, newer); got >= 0 {
		t.Errorf("compare(older, newer) = %d, want negative", got)
	}

	if got := s.Compare(newer, newer); got != 0 {
		t.Errorf("compare(newer, newer) = %d, want 0", got)
	}

	if got := s.Format(older); got != dateV1 {
		t.Errorf("format = %q, want %s", got, dateV1)
	}
}

// TestDatesRejectsNonDates pins the parse failures a client would see:
// wrong layout, impossible dates, and free text all refuse.
func TestDatesRejectsNonDates(t *testing.T) {
	s := version.Dates()

	for _, raw := range []string{"2024-6-1", "2024-13-40", "banana", ""} {
		if _, err := s.Parse(raw); err == nil {
			t.Errorf("parse(%q): want an error, got nil", raw)
		}
	}
}

// TestSequentialAcceptsBareAndPrefixed checks "v2", "V2", and "2" all mean
// version two, and that Format always renders the prefix.
func TestSequentialAcceptsBareAndPrefixed(t *testing.T) {
	s := version.Sequential()

	for _, raw := range []string{"v2", "V2", "2"} {
		got, err := s.Parse(raw)
		if err != nil {
			t.Fatalf("parse(%q): %v", raw, err)
		}

		if got != 2 {
			t.Errorf("parse(%q) = %d, want 2", raw, got)
		}
	}

	if got := s.Format(3); got != "v3" {
		t.Errorf("format(3) = %q, want v3", got)
	}
}

// TestSequentialRejectsNonNumbers pins the shapes that must not parse:
// signs, decimals, empty, a bare prefix, and a double prefix.
func TestSequentialRejectsNonNumbers(t *testing.T) {
	s := version.Sequential()

	for _, raw := range []string{"", "v", "vv2", "-1", "+3", "1.2", "two"} {
		if _, err := s.Parse(raw); err == nil {
			t.Errorf("parse(%q): want an error, got nil", raw)
		}
	}
}

// TestOrderedBuildsASchemeFromAParseFunc checks the cmp.Ordered shortcut:
// strconv.Atoi alone yields a working int scheme.
func TestOrderedBuildsASchemeFromAParseFunc(t *testing.T) {
	s := version.Ordered(strconv.Atoi)

	v, err := s.Parse("342")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if v != 342 {
		t.Errorf("parse = %d, want 342", v)
	}

	if got := s.Compare(9, 10); got >= 0 {
		t.Errorf("compare(9, 10) = %d, want negative", got)
	}

	if got := s.Format(342); got != "342" {
		t.Errorf("format = %q, want 342", got)
	}
}

// TestSchemesRoundTripThroughFormat checks Parse(Format(v)) compares equal
// to v for each built-in, the consistency Scheme's contract asks for.
func TestSchemesRoundTripThroughFormat(t *testing.T) {
	dates := version.Dates()

	d, err := dates.Parse(dateV2)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	d2, err := dates.Parse(dates.Format(d))
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}

	if dates.Compare(d, d2) != 0 {
		t.Errorf("dates round trip: %v != %v", d, d2)
	}

	seq := version.Sequential()

	n, err := seq.Parse(seq.Format(7))
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}

	if n != 7 {
		t.Errorf("sequential round trip = %d, want 7", n)
	}
}
