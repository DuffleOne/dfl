package http_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	dflhttp "github.com/duffleone/dfl/http"
)

// csv is a field type the built-in setters reject: it's a slice, and it has
// no UnmarshalText, so binding it needs an entry in RequestParser.Setters.
type csv []string

type tagsReq struct {
	Tags csv `query:"tags"`
}

// TestSettersBindsOtherwiseUnsupportedType checks a custom setter takes over
// for its type, letting a Req bind a field shape the parser rejects by default.
func TestSettersBindsOtherwiseUnsupportedType(t *testing.T) {
	p := &dflhttp.RequestParser{
		Setters: map[reflect.Type]dflhttp.SetterFunc{
			reflect.TypeFor[csv](): func(dst reflect.Value, raw string) error {
				dst.Set(reflect.ValueOf(csv(strings.Split(raw, ","))))

				return nil
			},
		},
	}

	got, err := p.Parse[tagsReq](httptest.NewRequest(http.MethodGet, "/?tags=a,b,c", nil))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := csv{"a", "b", "c"}
	if !slices.Equal(got.Tags, want) {
		t.Errorf("tags = %v, want %v", got.Tags, want)
	}
}

// TestUnsupportedTypeWithoutSetterFailsPrepare is the other half: without the
// setter the same Req is rejected at registration rather than at request time.
func TestUnsupportedTypeWithoutSetterFailsPrepare(t *testing.T) {
	p := &dflhttp.RequestParser{}

	if err := p.PrepareFor[tagsReq](); err == nil {
		t.Fatal("PrepareFor: want an error for the unsupported field type, got nil")
	}
}

type noteReq struct {
	ID   string `path:"id"`
	Note string `json:"note"`
}

// TestDecodeBodyReplacesJSONBinding checks a custom body decoder owns the body
// outright: it sees a non-JSON content type without the 415 the default path
// would return, and path fields are already bound by the time it runs.
func TestDecodeBodyReplacesJSONBinding(t *testing.T) {
	p := &dflhttp.RequestParser{
		DecodeBody: func(body io.Reader, dst any) error {
			raw, err := io.ReadAll(body)
			if err != nil {
				return err
			}

			dst.(*noteReq).Note = strings.ToUpper(string(raw))

			return nil
		},
	}

	r := httptest.NewRequest(http.MethodPost, "/widgets/w1", strings.NewReader("hello"))
	r.SetPathValue("id", "w1")
	r.Header.Set("Content-Type", "text/plain")

	got, err := p.Parse[noteReq](r)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.ID != "w1" {
		t.Errorf("id = %q, want %q", got.ID, "w1")
	}

	if got.Note != "HELLO" {
		t.Errorf("note = %q, want %q", got.Note, "HELLO")
	}
}

// TestDefaultParserRejectsNonJSONBody pins the behaviour DecodeBody opts out
// of, so the test above is demonstrably testing the hook and not a no-op.
func TestDefaultParserRejectsNonJSONBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/widgets/w1", strings.NewReader("hello"))
	r.Header.Set("Content-Type", "text/plain")

	_, err := dflhttp.DefaultRequestParser.Parse[noteReq](r)
	if err == nil {
		t.Fatal("parse: want an error for a non-JSON body, got nil")
	}

	var reqErr *dflhttp.ReqError
	if !errors.As(err, &reqErr) || reqErr.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("err = %v, want a 415 ReqError", err)
	}
}
