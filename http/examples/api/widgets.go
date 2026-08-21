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
// with a Reason per failure, so the client can fix everything in one
// round trip rather than one error at a time.
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
	rg.Handle(http.MethodPost, "/widgets/{id}", w.handleCreate)
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

// validate walks every field and records a Reason per failure, in the
// same {in, field} shape the binder emits, so the client reads one
// contract whether a field failed to bind or failed a domain rule. It
// deliberately doesn't bail on the first error.
func (r *CreateWidgetReq) validate() []dflhttp.Reason {
	var reasons []dflhttp.Reason

	invalid := func(in, field, msg string) {
		reasons = append(reasons, dflhttp.Reason{
			Code: "invalid",
			Meta: dflhttp.M{"in": in, "field": field, "error": msg},
		})
	}

	if r.ID <= 0 {
		invalid("path", "id", "must be a positive integer")
	}

	if r.Qty < 1 || r.Qty > 100 {
		invalid("query", "qty", "must be between 1 and 100")
	}

	switch {
	case strings.TrimSpace(r.Name) == "":
		reasons = append(reasons, dflhttp.Reason{
			Code: "required",
			Meta: dflhttp.M{"in": "body", "field": "name"},
		})
	case len(r.Name) > 50:
		invalid("body", "name", "must be at most 50 characters")
	}

	if _, ok := allowedColors[r.Color]; !ok {
		invalid("body", "color", fmt.Sprintf("must be one of red, blue, green (got %q)", r.Color))
	}

	return reasons
}

func (w *Widgets) handleCreate(_ context.Context, req *CreateWidgetReq) (*Widget, error) {
	if reasons := req.validate(); len(reasons) > 0 {
		return nil, dflhttp.New("validation_failed", nil).WithReasons(reasons...)
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
