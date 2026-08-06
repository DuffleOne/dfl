// Example program: WithErrorWriter hands the error response to the caller;
// this service emits its own envelope, dfl's shape never reaching the wire.
//
// Run:
//
//	go run ./http/examples/errorwriter
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	dflhttp "github.com/duffleone/dfl/http"
)

// apiError stands in for whatever error type a codebase already has (an
// in-house errors package, RFC 7807 problems, and so on). It knows its own
// HTTP status and carries structured reasons. dfl never sees any of this:
// the writer below owns the whole mapping.
type apiError struct {
	Code    string   `json:"code"`
	Status  int      `json:"-"`
	Reasons []reason `json:"reasons,omitempty"`
}

type reason struct {
	Field string `json:"field"`
	Msg   string `json:"msg"`
}

func (e *apiError) Error() string { return e.Code }

// writeError is the ErrorWriter. Own errors go out verbatim, reasons and
// all; dfl's binding failures (*ReqError) are translated into the same
// envelope so the API speaks exactly one error shape. A writer that's happy
// to emit dfl's shape for unrecognised errors could instead keep
// dflhttp.DefaultErrorWriter(nil) around as its fallback.
func writeError(w http.ResponseWriter, _ *http.Request, err error) {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		writeJSON(w, apiErr.Status, apiErr)

		return
	}

	var reqErr *dflhttp.ReqError
	if errors.As(err, &reqErr) {
		out := &apiError{Code: reqErr.Code, Status: reqErr.StatusCode()}

		if param, ok := reqErr.Meta["param"].(string); ok {
			out.Reasons = []reason{{Field: param, Msg: "could not be parsed"}}
		}

		writeJSON(w, out.Status, out)

		return
	}

	writeJSON(w, http.StatusInternalServerError, &apiError{Code: "internal"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// GetTeamReq binds the slug from the path and an optional size filter from
// the query. A non-integer size is a binding failure, which also flows
// through the custom writer.
type GetTeamReq struct {
	Slug string `path:"slug"`
	Size int    `query:"size"`
}

// Team is the resource shape on the wire.
type Team struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

var teams = map[string]Team{
	"platform": {Slug: "platform", Name: "Platform"},
}

func handleGet(_ context.Context, req *GetTeamReq) (*Team, error) {
	if reasons := validateSlug(req.Slug); len(reasons) > 0 {
		return nil, &apiError{Code: "invalid_slug", Status: http.StatusUnprocessableEntity, Reasons: reasons}
	}

	team, ok := teams[req.Slug]
	if !ok {
		return nil, &apiError{Code: "team_not_found", Status: http.StatusNotFound}
	}

	return &team, nil
}

// validateSlug collects every problem rather than stopping at the first, so
// the response's reasons give the client the full picture in one round trip.
func validateSlug(slug string) []reason {
	var reasons []reason

	if strings.ContainsAny(slug, " \t") {
		reasons = append(reasons, reason{Field: "slug", Msg: "must not contain whitespace"})
	}

	if slug != strings.ToLower(slug) {
		reasons = append(reasons, reason{Field: "slug", Msg: "must be lowercase"})
	}

	return reasons
}

func main() {
	r := dflhttp.NewRouter(http.NewServeMux(), dflhttp.WithErrorWriter(writeError))

	r.Handle(http.MethodGet, "/teams/{slug}", handleGet)

	addr := ":8080"

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
