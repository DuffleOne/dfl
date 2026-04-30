package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	dflhttp "github.com/duffleone/dfl/http"
)

// Widget is the resource shape on the wire.
type Widget struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Qty   int    `json:"qty"`
}

// Widgets demonstrates a handler that validates input from all three
// sources (path, query, body) in a single pass and returns one ReqError
// with a field-level breakdown of every failure, so the client can fix
// everything in one round trip rather than one error at a time.
type Widgets struct {
	mu    sync.Mutex
	store map[int]Widget
}

// NewWidgets returns a Widgets with an empty in-memory store.
func NewWidgets() *Widgets {
	return &Widgets{store: map[int]Widget{}}
}

// Mount wires up widget endpoints on rg.
func (w *Widgets) Mount(rg *dflhttp.Router) {
	dflhttp.Handle(rg, http.MethodPost, "/widgets/{id}", w.handleCreate)
}

// CreateWidgetReq mixes all three input sources: a path param, a query
// param, and a JSON body. Each field has a validation rule attached in
// validate() below.
type CreateWidgetReq struct {
	ID    int    `path:"id"`
	Qty   int    `query:"qty"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

var allowedColors = map[string]struct{}{
	"red":   {},
	"blue":  {},
	"green": {},
}

// validate walks every field and records a message per failure. It
// deliberately doesn't bail on the first error so the client gets a full
// picture in one response.
func (r *CreateWidgetReq) validate() dflhttp.M {
	fields := dflhttp.M{}

	if r.ID <= 0 {
		fields["id"] = "must be a positive integer"
	}

	if r.Qty < 1 || r.Qty > 100 {
		fields["qty"] = "must be between 1 and 100"
	}

	switch {
	case strings.TrimSpace(r.Name) == "":
		fields["name"] = "is required"
	case len(r.Name) > 50:
		fields["name"] = "must be at most 50 characters"
	}

	if _, ok := allowedColors[r.Color]; !ok {
		fields["color"] = fmt.Sprintf("must be one of red, blue, green (got %q)", r.Color)
	}

	if len(fields) == 0 {
		return nil
	}

	return fields
}

func (w *Widgets) handleCreate(_ context.Context, req *CreateWidgetReq) (*Widget, error) {
	if fields := req.validate(); fields != nil {
		return nil, dflhttp.New(http.StatusBadRequest, "validation_failed", dflhttp.M{
			"fields": fields,
		})
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	widget := Widget{
		ID:    req.ID,
		Qty:   req.Qty,
		Name:  req.Name,
		Color: req.Color,
	}
	w.store[widget.ID] = widget

	return &widget, nil
}
