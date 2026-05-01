package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ReqError is the canonical http error type. Code, StatusCode, and Meta are
// serialised on the wire; Reasons is internal-only, traversed by errors.Is
// and errors.As.
type ReqError struct {
	Code       string  `json:"code"`
	StatusCode int     `json:"status_code"`
	Meta       M       `json:"meta,omitempty"`
	Reasons    []error `json:"-"`
}

var _ error = (*ReqError)(nil)

// New builds a ReqError. reasons (if any) are recorded for errors.Is and
// errors.As traversal.
func New(statusCode int, code string, meta M, reasons ...error) *ReqError {
	return &ReqError{
		StatusCode: statusCode,
		Code:       code,
		Meta:       meta,
		Reasons:    reasons,
	}
}

// Wrap builds a ReqError that wraps err as its primary cause. Additional
// reasons are recorded after.
func Wrap(err error, statusCode int, code string, meta M, reasons ...error) *ReqError {
	all := make([]error, 0, 1+len(reasons))
	all = append(all, err)
	all = append(all, reasons...)

	return &ReqError{
		StatusCode: statusCode,
		Code:       code,
		Meta:       meta,
		Reasons:    all,
	}
}

func (e *ReqError) Error() string {
	return fmt.Sprintf("%s keys=%s", e.Code, strings.Join(e.Meta.Keys(), ", "))
}

// Unwrap returns the primary cause for single-step traversal via errors.Unwrap.
func (e *ReqError) Unwrap() error {
	if len(e.Reasons) == 0 {
		return nil
	}

	return e.Reasons[0]
}

// DefaultCoercer is the minimal Coercer: it returns *ReqError as-is (via
// errors.As) and otherwise wraps err as 500 "unknown". It does not know
// about samber/oops or any other third-party error type. For oops support
// see github.com/duffleone/dfl/http/oops.
func DefaultCoercer(err error) *ReqError {
	if err == nil {
		return nil
	}

	var reqErr *ReqError
	if errors.As(err, &reqErr) {
		return reqErr
	}

	return Wrap(err, http.StatusInternalServerError, "unknown", nil)
}
