package pgxdb

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

const emailKey = "users_email_key"

// TestClassifySentinels pins the SQLSTATE-to-sentinel mapping: every
// constraint-class code gets its sentinel, deadlock rides with
// serialization failure, and unrecognised codes pass through unwrapped.
func TestClassifySentinels(t *testing.T) {
	cases := []struct {
		name string
		code string
		want error
	}{
		{"unique violation", pgerrcode.UniqueViolation, ErrUniqueViolation},
		{"foreign key violation", pgerrcode.ForeignKeyViolation, ErrForeignKeyViolation},
		{"check violation", pgerrcode.CheckViolation, ErrCheckViolation},
		{"not null violation", pgerrcode.NotNullViolation, ErrNotNullViolation},
		{"exclusion violation", pgerrcode.ExclusionViolation, ErrExclusionViolation},
		{"serialization failure", pgerrcode.SerializationFailure, ErrSerializationFailure},
		{"deadlock maps to serialization failure", pgerrcode.DeadlockDetected, ErrSerializationFailure},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pgErr := &pgconn.PgError{Code: tc.code, ConstraintName: emailKey}

			got := classify(pgErr)
			if !errors.Is(got, tc.want) {
				t.Errorf("classify(%s) does not match %v", tc.code, tc.want)
			}

			// The original driver error must stay reachable for the
			// constraint name and detail.
			inner, ok := errors.AsType[*pgconn.PgError](got)
			if !ok || inner.ConstraintName != emailKey {
				t.Errorf("classify lost the underlying *pgconn.PgError")
			}
		})
	}
}

// TestClassifyPassthrough: nil, plain errors, NotFound, and PgErrors with
// codes outside the constraint class all come back exactly as they went in.
func TestClassifyPassthrough(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"plain error", errors.New("nope")},
		{"NotFound", NotFound},
		{"unrelated SQLSTATE", &pgconn.PgError{Code: pgerrcode.UndefinedTable}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classify(tc.err); !errors.Is(got, tc.err) || (tc.err != nil && got != tc.err) {
				t.Errorf("classify(%v) = %v, want it untouched", tc.err, got)
			}
		})
	}
}

// TestClassifyIdempotent: an error that already carries its sentinel is not
// wrapped a second time, so classify can sit on every return path without
// stacking prefixes.
func TestClassifyIdempotent(t *testing.T) {
	once := classify(&pgconn.PgError{Code: pgerrcode.UniqueViolation})

	twice := classify(once)
	if twice != once { //nolint:errorlint // identity is the point: no re-wrap
		t.Errorf("classify reclassified an already-classified error: %v", twice)
	}
}

// TestConstraintName covers the three shapes it meets: a raw driver error,
// one wrapped by classify, and an error with no PgError at all.
func TestConstraintName(t *testing.T) {
	pgErr := &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: emailKey}

	if got := ConstraintName(pgErr); got != emailKey {
		t.Errorf("ConstraintName(raw) = %q, want users_email_key", got)
	}

	if got := ConstraintName(classify(pgErr)); got != emailKey {
		t.Errorf("ConstraintName(classified) = %q, want users_email_key", got)
	}

	if got := ConstraintName(errors.New("nope")); got != "" {
		t.Errorf("ConstraintName(plain) = %q, want empty", got)
	}
}

// TestIsUniqueViolation: the code must match, and the constraint name must
// match when given; an empty constraint matches any unique violation. The
// classified form has to keep matching, since that's what the helpers
// actually return.
func TestIsUniqueViolation(t *testing.T) {
	emailClash := &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: emailKey}

	cases := []struct {
		name       string
		err        error
		constraint string
		want       bool
	}{
		{"matching constraint", emailClash, emailKey, true},
		{"other constraint", emailClash, "users_external_id_key", false},
		{"empty constraint matches any", emailClash, "", true},
		{"classified error still matches", classify(emailClash), emailKey, true},
		{"wrapped error still matches", fmt.Errorf("create user: %w", emailClash), emailKey, true},
		{"wrong code", &pgconn.PgError{Code: pgerrcode.CheckViolation}, "", false},
		{"no PgError", errors.New("nope"), "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsUniqueViolation(tc.err, tc.constraint); got != tc.want {
				t.Errorf("IsUniqueViolation = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestIsExclusionViolation: 23P01 is its own code, invisible to
// unique-violation checks, and vice versa.
func TestIsExclusionViolation(t *testing.T) {
	overlap := &pgconn.PgError{Code: pgerrcode.ExclusionViolation, ConstraintName: "validity_no_overlap"}

	if !IsExclusionViolation(overlap, "validity_no_overlap") {
		t.Error("IsExclusionViolation did not match its own constraint")
	}

	if IsUniqueViolation(overlap, "") {
		t.Error("IsUniqueViolation matched an exclusion violation")
	}

	if IsExclusionViolation(&pgconn.PgError{Code: pgerrcode.UniqueViolation}, "") {
		t.Error("IsExclusionViolation matched a unique violation")
	}
}
