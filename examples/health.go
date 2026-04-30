package main

import (
	"context"
	"net/http"

	dflhttp "github.com/duffleone/dfl/http"
)

// Health shows handlers with no input (Empty Req) and with primitive or
// empty responses.
type Health struct {
	GitCommitSHA string
	Version      string
}

// Mount wires up health endpoints on rg.
func (h Health) Mount(rg dflhttp.Router) {
	rg.Handle(http.MethodGet, "/health", h.handleHealth)
	rg.Handle(http.MethodGet, "/sha", h.handleSHA)
	rg.Handle(http.MethodGet, "/version", h.handleVersion)
	rg.Handle(http.MethodPost, "/ping", h.handlePing)
}

// String resp gets JSON-encoded as "up", with content-type application/json.
func (h Health) handleHealth(_ context.Context, _ dflhttp.Empty) (string, error) {
	return "up", nil
}

func (h Health) handleSHA(_ context.Context, _ dflhttp.Empty) (string, error) {
	return h.GitCommitSHA, nil
}

func (h Health) handleVersion(_ context.Context, _ dflhttp.Empty) (string, error) {
	return h.Version, nil
}

// Empty Resp produces a 204 No Content response with an empty body.
func (h Health) handlePing(_ context.Context, _ dflhttp.Empty) (res *dflhttp.Empty, err error) {
	return res, err
}
