package cher

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	dflhttp "github.com/duffleone/dfl/http"
	mojocher "github.com/wearemojo/mojo-public-go/lib/cher"
)

// TestCoerceNil checks the trivial case.
func TestCoerceNil(t *testing.T) {
	if got := Coerce(nil); got != nil {
		t.Errorf("Coerce(nil) = %v, want nil", got)
	}
}

// TestCoercePassesThroughReqError: a *ReqError on the way in is returned
// unchanged (same instance), so a handler reaching for dflhttp.New directly
// keeps its status, code, and meta intact.
func TestCoercePassesThroughReqError(t *testing.T) {
	in := dflhttp.New("missing", dflhttp.M{"id": "x"})

	if got := Coerce(in); got != in {
		t.Errorf("Coerce should return the same *ReqError instance")
	}
}

// TestCoerceUnwrapsWrappedReqError: errors.As finds a *ReqError even when
// something else has wrapped it.
func TestCoerceUnwrapsWrappedReqError(t *testing.T) {
	in := dflhttp.New("upstream", nil)
	wrapped := fmt.Errorf("layer: %w", in)

	if got := Coerce(wrapped); got != in {
		t.Errorf("Coerce should unwrap to the inner *ReqError")
	}
}

// TestCoerceStatusCodes walks cher's code-to-status table. The default is 400,
// not 500: cher treats an error somebody wrote down as the client's problem
// unless the code says otherwise.
func TestCoerceStatusCodes(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{mojocher.BadRequest, http.StatusBadRequest},
		{mojocher.Unauthorized, http.StatusUnauthorized},
		{mojocher.AccessDenied, http.StatusForbidden},
		{mojocher.NotFound, http.StatusNotFound},
		{mojocher.RouteNotFound, http.StatusNotFound},
		{mojocher.MethodNotAllowed, http.StatusMethodNotAllowed},
		{mojocher.EndpointWithdrawn, http.StatusGone},
		{mojocher.TooManyRequests, http.StatusTooManyRequests},
		{mojocher.Unknown, http.StatusInternalServerError},
		{mojocher.CoercionError, http.StatusInternalServerError},
		{mojocher.RequestTimeout, http.StatusInternalServerError},
		{mojocher.ThirdPartyTimeout, http.StatusBadRequest},
		{mojocher.ContextCanceled, http.StatusBadRequest},
		{"anything_service_specific", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			out := Coerce(mojocher.New(tc.code, nil))

			if out.StatusCode() != tc.want {
				t.Errorf("StatusCode = %d, want %d", out.StatusCode(), tc.want)
			}

			if out.Code != tc.code {
				t.Errorf("Code = %q, want %q", out.Code, tc.code)
			}
		})
	}
}

// TestCoerceBlankCode: a cher error with no code would otherwise serialise an
// empty code with cher's 400 default. It becomes unknown, and the status is
// then derived from that, so it lands on 500.
func TestCoerceBlankCode(t *testing.T) {
	cases := []struct{ name, code string }{
		{"empty", ""},
		{"whitespace only", "  \t "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Coerce(mojocher.New(tc.code, nil))

			if out.Code != mojocher.Unknown {
				t.Errorf("Code = %q, want %q", out.Code, mojocher.Unknown)
			}

			if out.StatusCode() != http.StatusInternalServerError {
				t.Errorf("StatusCode = %d, want 500", out.StatusCode())
			}
		})
	}
}

// TestCoerceTrimsCode: surrounding whitespace is trimmed off the code, and the
// status follows the trimmed value rather than falling through to 400.
func TestCoerceTrimsCode(t *testing.T) {
	out := Coerce(mojocher.New("  not_found  ", nil))

	if out.Code != mojocher.NotFound {
		t.Errorf("Code = %q, want %q", out.Code, mojocher.NotFound)
	}

	if out.StatusCode() != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", out.StatusCode())
	}
}

// TestCoerceMeta: meta carries over key for key.
func TestCoerceMeta(t *testing.T) {
	out := Coerce(mojocher.New(mojocher.NotFound, mojocher.M{"id": "42", "kind": "user"}))

	if got, _ := out.Meta["id"].(string); got != "42" {
		t.Errorf("Meta[id] = %v, want %q", out.Meta["id"], "42")
	}

	if got, _ := out.Meta["kind"].(string); got != "user" {
		t.Errorf("Meta[kind] = %v, want %q", out.Meta["kind"], "user")
	}
}

// TestCoerceEmptyMetaIsNil: an absent or empty cher meta must not become an
// empty dflhttp.M, or `meta,omitempty` would stop dropping the key and every
// error body would carry "meta":{}.
func TestCoerceEmptyMetaIsNil(t *testing.T) {
	cases := []struct {
		name string
		meta mojocher.M
	}{
		{"nil meta", nil},
		{"empty meta", mojocher.M{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if out := Coerce(mojocher.New(mojocher.NotFound, tc.meta)); out.Meta != nil {
				t.Errorf("Meta = %v, want nil", out.Meta)
			}
		})
	}
}

// TestCoerceCopiesMeta: the ReqError must not share its meta map with the cher
// error it came from, in either direction. cher errors get passed around and
// re-wrapped, and a shared map would let that mutate a response body.
func TestCoerceCopiesMeta(t *testing.T) {
	meta := mojocher.M{"id": "42"}
	out := Coerce(mojocher.New(mojocher.NotFound, meta))

	meta["id"] = "mutated"

	if got, _ := out.Meta["id"].(string); got != "42" {
		t.Errorf("Meta[id] = %v after mutating the source, want %q", out.Meta["id"], "42")
	}

	out.Meta["id"] = "mutated back"

	if got, _ := meta["id"].(string); got != "mutated" {
		t.Errorf("source meta[id] = %v after mutating the ReqError, want %q", meta["id"], "mutated")
	}
}

// TestCoerceReasons: both sides nest, so the tree survives the projection
// rather than being flattened into one list where the client would have to
// guess which failed check belongs to which.
func TestCoerceReasons(t *testing.T) {
	err := mojocher.New(mojocher.BadRequest, nil,
		mojocher.New("required", mojocher.M{"field": "name"}),
		mojocher.New("invalid", mojocher.M{"field": "size"},
			mojocher.New("too_small", mojocher.M{"min": 1}),
		),
	)

	out := Coerce(err)

	if len(out.Reasons) != 2 {
		t.Fatalf("got %d top-level reasons (%v), want 2", len(out.Reasons), out.Reasons)
	}

	if out.Reasons[0].Code != "required" {
		t.Errorf("Reasons[0].Code = %q, want required", out.Reasons[0].Code)
	}

	if got, _ := out.Reasons[0].Meta["field"].(string); got != "name" {
		t.Errorf("Reasons[0].Meta[field] = %v, want %q", out.Reasons[0].Meta["field"], "name")
	}

	if out.Reasons[0].Reasons != nil {
		t.Errorf("Reasons[0].Reasons = %v, want nil for a leaf", out.Reasons[0].Reasons)
	}

	child := out.Reasons[1]
	if child.Code != "invalid" {
		t.Errorf("Reasons[1].Code = %q, want invalid", child.Code)
	}

	if len(child.Reasons) != 1 {
		t.Fatalf("Reasons[1].Reasons = %v, want one nested reason", child.Reasons)
	}

	if child.Reasons[0].Code != "too_small" {
		t.Errorf("nested code = %q, want too_small", child.Reasons[0].Code)
	}

	if got, _ := child.Reasons[0].Meta["min"].(int); got != 1 {
		t.Errorf("nested Meta[min] = %v, want 1", child.Reasons[0].Meta["min"])
	}
}

// TestCoerceDeepReasons: nesting isn't capped at one level, and an empty
// child list stays nil so `reasons,omitempty` drops the key on leaves.
func TestCoerceDeepReasons(t *testing.T) {
	err := mojocher.New(mojocher.BadRequest, nil,
		mojocher.New("a", nil,
			mojocher.New("b", nil,
				mojocher.New("c", nil),
			),
		),
	)

	body, marshalErr := json.Marshal(Coerce(err))
	if marshalErr != nil {
		t.Fatalf("marshalling: %v", marshalErr)
	}

	want := `{"code":"bad_request","reasons":[{"code":"a","reasons":` +
		`[{"code":"b","reasons":[{"code":"c"}]}]}]}`

	if string(body) != want {
		t.Errorf("body = %s,\nwant %s", body, want)
	}
}

// TestCoerceNoReasons: an error without reasons leaves Reasons nil, so
// `reasons,omitempty` drops the key.
func TestCoerceNoReasons(t *testing.T) {
	if out := Coerce(mojocher.New(mojocher.NotFound, nil)); out.Reasons != nil {
		t.Errorf("Reasons = %v, want nil", out.Reasons)
	}
}

// TestCoerceWrappedCherError: cher errors travel wrapped (its own helpers use
// pkg/errors.Wrap), so the projection has to work through a chain.
func TestCoerceWrappedCherError(t *testing.T) {
	inner := mojocher.New(mojocher.AccessDenied, mojocher.M{"scope": "admin"})
	wrapped := fmt.Errorf("checking permissions: %w", inner)

	out := Coerce(wrapped)

	if out.Code != mojocher.AccessDenied {
		t.Errorf("Code = %q, want %q", out.Code, mojocher.AccessDenied)
	}

	if out.StatusCode() != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want 403", out.StatusCode())
	}

	if got, _ := out.Meta["scope"].(string); got != "admin" {
		t.Errorf("Meta[scope] = %v, want %q", out.Meta["scope"], "admin")
	}
}

// TestCoercePreservesCause: the original error stays on the ReqError's cause
// chain, so handlers and logs can still errors.As their way back to it.
func TestCoercePreservesCause(t *testing.T) {
	inner := mojocher.New(mojocher.NotFound, nil)

	out := Coerce(fmt.Errorf("loading user: %w", inner))

	var got mojocher.E
	if !errors.As(out, &got) {
		t.Fatalf("errors.As should recover the cher error from %v", out)
	}

	if got.Code != mojocher.NotFound {
		t.Errorf("recovered code = %q, want %q", got.Code, mojocher.NotFound)
	}
}

// TestCoercePlainError: anything that isn't a cher error or a ReqError is a
// 500 unknown with no meta. The message stays on the cause chain rather than
// going in the body, since an unclassified error's text is as likely to be a
// driver message as something a client should read.
func TestCoercePlainError(t *testing.T) {
	inner := errors.New("dial tcp 10.0.0.1:5432: connection refused")

	out := Coerce(fmt.Errorf("querying: %w", inner))

	if out.StatusCode() != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", out.StatusCode())
	}

	if out.Code != "unknown" {
		t.Errorf("Code = %q, want %q", out.Code, "unknown")
	}

	if out.Meta != nil {
		t.Errorf("Meta = %v, want nil", out.Meta)
	}

	if !errors.Is(out, inner) {
		t.Errorf("errors.Is should find the original error on the cause chain")
	}
}

// TestCoerceDropsExtra: Extra holds unrecognised keys from an upstream
// service's error body, kept for log forensics. It must not reach our own
// callers.
func TestCoerceDropsExtra(t *testing.T) {
	var cherErr mojocher.E
	if err := json.Unmarshal([]byte(`{"code":"not_found","trace_id":"abc"}`), &cherErr); err != nil {
		t.Fatalf("unmarshalling the upstream body: %v", err)
	}

	if cherErr.Extra["trace_id"] != "abc" {
		t.Fatalf("cher should have captured trace_id in Extra, got %v", cherErr.Extra)
	}

	out := Coerce(cherErr)

	if _, ok := out.Meta["trace_id"]; ok {
		t.Errorf("Meta = %v, should not carry Extra keys", out.Meta)
	}
}

// TestCoerceWireShape pins the JSON a coerced cher error actually writes,
// which is what the caller sees.
func TestCoerceWireShape(t *testing.T) {
	err := mojocher.New(mojocher.BadRequest, mojocher.M{"resource": "team"},
		mojocher.New("required", mojocher.M{"field": "name"}),
	)

	body, marshalErr := json.Marshal(Coerce(err))
	if marshalErr != nil {
		t.Fatalf("marshalling: %v", marshalErr)
	}

	want := `{"code":"bad_request","meta":{"resource":"team"},` +
		`"reasons":[{"code":"required","meta":{"field":"name"}}]}`

	if string(body) != want {
		t.Errorf("body = %s,\nwant %s", body, want)
	}
}
