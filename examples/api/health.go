// Package api hosts the example handlers shared between the std and chi
// example programs. It's deliberately router-agnostic: handlers take
// *dflhttp.Router and don't know which mux it's wrapping.
//
// Convention used by every handler in this package: both Req and Resp are
// pointer types. That way handlers can return (nil, err) cleanly on the
// error path, and Empty-style routes get a nil req for free.
package api

import (
	"context"
	"net/http"

	dflhttp "github.com/duffleone/dfl/http"
)

// Health shows handlers with no input (*Empty) and with simple struct
// responses for what would otherwise be primitive returns.
type Health struct {
	GitCommitSHA string
	Version      string
}

// Mount wires up health endpoints on rg.
func (h Health) Mount(rg *dflhttp.Router) {
	dflhttp.Handle(rg, http.MethodGet, "/health", h.handleHealth)
	dflhttp.Handle(rg, http.MethodGet, "/sha", h.handleSHA)
	dflhttp.Handle(rg, http.MethodGet, "/version", h.handleVersion)
	dflhttp.Handle(rg, http.MethodPost, "/ping", h.handlePing)
}

// StatusResp is the common shape for the simple health endpoints. A pointer
// to it (rather than a bare string) keeps the convention that all responses
// are pointer-to-struct.
type StatusResp struct {
	Status string `json:"status"`
}

func (h Health) handleHealth(_ context.Context, _ *dflhttp.Empty) (*StatusResp, error) {
	return &StatusResp{Status: "up"}, nil
}

func (h Health) handleSHA(_ context.Context, _ *dflhttp.Empty) (*StatusResp, error) {
	return &StatusResp{Status: h.GitCommitSHA}, nil
}

func (h Health) handleVersion(_ context.Context, _ *dflhttp.Empty) (*StatusResp, error) {
	return &StatusResp{Status: h.Version}, nil
}

// Empty Resp produces a 204 No Content response with an empty body. Whether
// the pointer is nil or non-nil makes no difference: adapt only looks at
// the Resp type, not the value, when deciding to write 204.
func (h Health) handlePing(_ context.Context, _ *dflhttp.Empty) (*dflhttp.Empty, error) {
	return &dflhttp.Empty{}, nil
}
