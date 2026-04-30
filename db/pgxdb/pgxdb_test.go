package pgxdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// Tests in this package cover the pure logic that doesn't need a database:
// error classification (isSerializationFailure) and the retry loop
// (retrySerializable). The DB methods that actually talk to postgres
// (Tx, TxRead, Get, Select) need a live connection and are best covered by
// integration tests against a real or containerised database.

func init() {
	// Quiet down the retry warn logs so test output stays readable.
	slog.SetDefault(slog.New(slog.DiscardHandler))
}

// TestIsSerializationFailure pins down which errors are treated as
// retry-eligible. Only a *pgconn.PgError with the SerializationFailure
// SQLSTATE code counts; everything else is non-retryable, even if it
// contains "serial" in the message somewhere.
func TestIsSerializationFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			"nil is not a serialization failure",
			nil,
			false,
		},
		{
			"plain error is not a serialization failure",
			errors.New("nope"),
			false,
		},
		{
			"PgError with unrelated SQLSTATE is not a serialization failure",
			&pgconn.PgError{Code: pgerrcode.UniqueViolation},
			false,
		},
		{
			"PgError with SerializationFailure SQLSTATE is a serialization failure",
			&pgconn.PgError{Code: pgerrcode.SerializationFailure},
			true,
		},
		{
			"wrapped PgError still classifies via errors.As traversal",
			fmt.Errorf("layer: %w", &pgconn.PgError{Code: pgerrcode.SerializationFailure}),
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSerializationFailure(tc.err); got != tc.want {
				t.Errorf("isSerializationFailure = %v, want %v", got, tc.want)
			}
		})
	}
}

// scriptedOp returns an op that pops one error from script per call. Once
// drained, every further call panics, which is how we'd notice if the loop
// retries more times than the test expects.
func scriptedOp(t *testing.T, script []error, calls *int) func() error {
	t.Helper()

	return func() error {
		if *calls >= len(script) {
			t.Fatalf("retry loop made more attempts than the script provided (%d already)", *calls)
		}

		err := script[*calls]
		*calls++

		return err
	}
}

// TestRetrySerializableSucceedsFirstTry: the happy path. nil from op means
// we return nil on the very first attempt and don't loop.
func TestRetrySerializableSucceedsFirstTry(t *testing.T) {
	calls := 0
	op := scriptedOp(t, []error{nil}, &calls)

	if err := retrySerializable(t.Context(), 3, op); err != nil {
		t.Errorf("retrySerializable = %v, want nil", err)
	}

	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

// TestRetrySerializableRetriesThenSucceeds: two serialization failures
// followed by a success returns nil at the third attempt.
func TestRetrySerializableRetriesThenSucceeds(t *testing.T) {
	serFail := &pgconn.PgError{Code: pgerrcode.SerializationFailure}

	calls := 0
	op := scriptedOp(t, []error{serFail, serFail, nil}, &calls)

	if err := retrySerializable(t.Context(), 3, op); err != nil {
		t.Errorf("retrySerializable = %v, want nil", err)
	}

	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

// TestRetrySerializableNonSerialErrorReturnsImmediately: any other error
// breaks the loop on the first attempt.
func TestRetrySerializableNonSerialErrorReturnsImmediately(t *testing.T) {
	other := errors.New("other")

	calls := 0
	op := scriptedOp(t, []error{other}, &calls)

	err := retrySerializable(t.Context(), 3, op)
	if !errors.Is(err, other) {
		t.Errorf("retrySerializable = %v, want %v", err, other)
	}

	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on non-serial errors)", calls)
	}
}

// TestRetrySerializableExhaustsRetries: a persistent serialization failure
// should be retried maxRetries times then surfaced. With maxRetries=3 we
// expect 4 total calls (1 initial + 3 retries).
func TestRetrySerializableExhaustsRetries(t *testing.T) {
	serFail := &pgconn.PgError{Code: pgerrcode.SerializationFailure}

	calls := 0
	op := scriptedOp(t, []error{serFail, serFail, serFail, serFail}, &calls)

	err := retrySerializable(t.Context(), 3, op)
	if !errors.Is(err, serFail) {
		t.Errorf("retrySerializable = %v, want serFail", err)
	}

	if calls != 4 {
		t.Errorf("calls = %d, want 4 (1 initial + 3 retries)", calls)
	}
}

// TestRetrySerializableContextCancelledMidLoop: a cancelled context aborts
// the retry loop even when the next error would be retry-eligible.
func TestRetrySerializableContextCancelledMidLoop(t *testing.T) {
	serFail := &pgconn.PgError{Code: pgerrcode.SerializationFailure}

	ctx, cancel := context.WithCancel(t.Context())

	calls := 0
	op := func() error {
		calls++

		cancel()

		return serFail
	}

	err := retrySerializable(ctx, 3, op)
	if !errors.Is(err, serFail) {
		t.Errorf("retrySerializable = %v, want serFail", err)
	}

	if calls != 1 {
		t.Errorf("calls = %d, want 1 (cancelled context aborts retries)", calls)
	}
}

// TestRetrySerializableZeroRetries: with maxRetries=0 the loop runs the op
// exactly once and returns whatever it gets.
func TestRetrySerializableZeroRetries(t *testing.T) {
	serFail := &pgconn.PgError{Code: pgerrcode.SerializationFailure}

	calls := 0
	op := scriptedOp(t, []error{serFail}, &calls)

	err := retrySerializable(t.Context(), 0, op)
	if !errors.Is(err, serFail) {
		t.Errorf("retrySerializable = %v, want serFail", err)
	}

	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retries when maxRetries=0)", calls)
	}
}
