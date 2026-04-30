// Package oops provides a Coercer that understands samber/oops errors and
// projects them into *dflhttp.ReqError. Opt-in: callers that don't use
// samber/oops should use dflhttp.DefaultCoercer instead.
package oops

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"strings"

	dflhttp "github.com/duffleone/dfl/http"
	samberoops "github.com/samber/oops"
)

// Coerce projects err into a *dflhttp.ReqError. Order of attempts:
//   - nil in, nil out
//   - existing *dflhttp.ReqError (via errors.As)
//   - samber/oops error: extract Code, Context, Public; 500 status
//   - any error with a non-nil Unwrap chain: derive code from message; 500
//   - everything else: 500 "unknown"
func Coerce(err error) *dflhttp.ReqError {
	if err == nil {
		return nil
	}

	var reqErr *dflhttp.ReqError
	if errors.As(err, &reqErr) {
		return reqErr
	}

	var oopsErr samberoops.OopsError
	if errors.As(err, &oopsErr) {
		return coerceOops(err, oopsErr)
	}

	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		code := codeFromMessage(unwrapped.Error())
		if code == "" {
			code = "unknown"
		}

		return dflhttp.Wrap(err, http.StatusInternalServerError, code, nil)
	}

	return dflhttp.Wrap(err, http.StatusInternalServerError, "unknown", nil)
}

func coerceOops(err error, oopsErr samberoops.OopsError) *dflhttp.ReqError {
	code := normaliseOopsCode(oopsErr.Code())
	if code == "" {
		if u := errors.Unwrap(oopsErr); u != nil {
			code = codeFromMessage(u.Error())
		}
	}

	if code == "" {
		code = "unknown"
	}

	meta := dflhttp.M{"error": oopsErr.Error()}
	maps.Copy(meta, oopsErr.Context())

	if public := oopsErr.Public(); public != "" {
		meta["public"] = public
	}

	return dflhttp.Wrap(err, http.StatusInternalServerError, code, meta)
}

// normaliseOopsCode is defensive against samber/oops API drift. Whatever
// Code() returns (string today, possibly any in older or forked versions),
// stringify it and trim whitespace; empty means no code.
func normaliseOopsCode(c any) string {
	if c == nil {
		return ""
	}

	return strings.TrimSpace(fmt.Sprintf("%v", c))
}

var (
	codeCleanRe    = regexp.MustCompile(`[^a-z0-9_]+`)
	codeCollapseRe = regexp.MustCompile(`_+`)
	codeTrimRe     = regexp.MustCompile(`^_|_$`)
)

func codeFromMessage(msg string) string {
	s := strings.ToLower(strings.TrimSpace(msg))
	s = strings.ReplaceAll(s, " ", "_")
	s = codeCleanRe.ReplaceAllString(s, "")
	s = codeCollapseRe.ReplaceAllString(s, "_")
	s = codeTrimRe.ReplaceAllString(s, "")

	return s
}
