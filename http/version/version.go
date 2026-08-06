// Package version dispatches one route to different handlers by API
// version, so an endpoint can change shape without breaking pinned
// clients. A Scheme types and orders versions (Dates, Sequential,
// Ordered), Sources say where the version travels (Header, Query,
// PathValue, Default), and an Endpoint holds a handler per version.
// Stripe-style inheritance by default; the README covers the rest.
package version

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Scheme defines a version format: how raw strings parse into V, how two
// V values order, and how a V renders back for error metadata. It must be
// comparison-consistent: a value compares equal to itself after a
// Parse/Format round trip.
type Scheme[V any] interface {
	// Parse turns a raw wire value into a version. The error is surfaced
	// to the client (inside a 400 version_invalid response), so it should
	// say what a well-formed version looks like rather than reference
	// internals.
	Parse(raw string) (V, error)

	// Compare orders two versions: negative when a is older than b, zero
	// when equal, positive when a is newer.
	Compare(a, b V) int

	// Format renders v canonically.
	Format(v V) string
}

// Dates returns the Scheme for date-stamped versions like "2024-06-01",
// the Stripe model: every breaking change gets the date it shipped, and
// clients pin the date they integrated against. Values parse with
// time.DateOnly and order chronologically.
func Dates() Scheme[time.Time] {
	return dateScheme{}
}

type dateScheme struct{}

func (dateScheme) Parse(raw string) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("not a YYYY-MM-DD date: %q", raw)
	}

	return t, nil
}

func (dateScheme) Compare(a, b time.Time) int {
	return a.Compare(b)
}

func (dateScheme) Format(v time.Time) string {
	return v.Format(time.DateOnly)
}

// Sequential returns the Scheme for counted versions: "v1", "v2", "v3".
// The leading v is optional and case-insensitive on the wire, so "2" and
// "V2" both mean version two; Format always renders it.
func Sequential() Scheme[int] {
	return seqScheme{}
}

type seqScheme struct{}

func (seqScheme) Parse(raw string) (int, error) {
	digits := strings.TrimPrefix(strings.ToLower(raw), "v")
	if digits == "" {
		return 0, fmt.Errorf("not a version number like v2: %q", raw)
	}

	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a version number like v2: %q", raw)
		}
	}

	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("not a version number like v2: %q", raw)
	}

	return n, nil
}

func (seqScheme) Compare(a, b int) int {
	return cmp.Compare(a, b)
}

func (seqScheme) Format(v int) string {
	return "v" + strconv.Itoa(v)
}

// Ordered builds a Scheme for any ordered type from just a parse
// function, version.Ordered(strconv.Atoi) say: cmp.Compare orders values
// and fmt.Sprint formats them. The shortcut for versions that are
// naturally an int, float, or string, like an app build number. Anything
// needing its own ordering or rendering (semver) implements Scheme.
func Ordered[V cmp.Ordered](parse func(raw string) (V, error)) Scheme[V] {
	return orderedScheme[V]{parse: parse}
}

type orderedScheme[V cmp.Ordered] struct {
	parse func(string) (V, error)
}

func (s orderedScheme[V]) Parse(raw string) (V, error) {
	return s.parse(raw)
}

func (orderedScheme[V]) Compare(a, b V) int {
	return cmp.Compare(a, b)
}

func (orderedScheme[V]) Format(v V) string {
	return fmt.Sprint(v)
}
