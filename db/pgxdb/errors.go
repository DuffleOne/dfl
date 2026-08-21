package pgxdb

import (
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// Constraint-class sentinels. The generic helpers wrap driver errors with
// these, so errors.Is answers "which class" while errors.As still reaches
// the *pgconn.PgError underneath for the constraint name and detail. They
// are deliberately not HTTP-aware: what a violation means depends on which
// write hit it, so mapping one to a status stays the caller's job.
var (
	ErrUniqueViolation      = errors.New("pgxdb: unique violation")
	ErrForeignKeyViolation  = errors.New("pgxdb: foreign key violation")
	ErrCheckViolation       = errors.New("pgxdb: check violation")
	ErrNotNullViolation     = errors.New("pgxdb: not null violation")
	ErrExclusionViolation   = errors.New("pgxdb: exclusion violation")
	ErrSerializationFailure = errors.New("pgxdb: serialization failure")
)

// sentinelFor maps a constraint-class SQLSTATE onto its sentinel. Deadlock
// rides with serialization failure: both are transient and TxSerializable
// retries them the same way.
func sentinelFor(code string) (error, bool) {
	switch code {
	case pgerrcode.UniqueViolation:
		return ErrUniqueViolation, true
	case pgerrcode.ForeignKeyViolation:
		return ErrForeignKeyViolation, true
	case pgerrcode.CheckViolation:
		return ErrCheckViolation, true
	case pgerrcode.NotNullViolation:
		return ErrNotNullViolation, true
	case pgerrcode.ExclusionViolation:
		return ErrExclusionViolation, true
	case pgerrcode.SerializationFailure, pgerrcode.DeadlockDetected:
		return ErrSerializationFailure, true
	}

	return nil, false
}

// classify wraps err with its constraint-class sentinel when it carries a
// recognised SQLSTATE. Anything unrecognised, NotFound included, passes
// through untouched, and an error already carrying its sentinel is never
// wrapped twice.
func classify(err error) error {
	if err == nil {
		return nil
	}

	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return err
	}

	sentinel, found := sentinelFor(pgErr.Code)
	if !found || errors.Is(err, sentinel) {
		return err
	}

	return fmt.Errorf("%w: %w", sentinel, err)
}

// ConstraintName returns the name of the constraint err violated, or ""
// when err carries no *pgconn.PgError or the server sent no name.
func ConstraintName(err error) string {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return ""
	}

	return pgErr.ConstraintName
}

// IsUniqueViolation reports whether err is a unique violation (23505) and,
// when constraint is non-empty, that it names that constraint. A table with
// several unique indexes raises the same SQLSTATE for each, so the useful
// question names the index, not the class. Works on raw driver errors too,
// not just ones the helpers classified.
func IsUniqueViolation(err error, constraint string) bool {
	return isViolation(err, pgerrcode.UniqueViolation, constraint)
}

// IsExclusionViolation is IsUniqueViolation for EXCLUDE constraints, which
// raise 23P01 rather than 23505 and are invisible to unique-violation
// checks; a tstzrange overlap exclusion is the classic case.
func IsExclusionViolation(err error, constraint string) bool {
	return isViolation(err, pgerrcode.ExclusionViolation, constraint)
}

func isViolation(err error, code, constraint string) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok || pgErr.Code != code {
		return false
	}

	return constraint == "" || pgErr.ConstraintName == constraint
}
