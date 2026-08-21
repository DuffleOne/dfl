package http

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// SetterFunc parses a raw path or query string into a single struct field.
// dst is the addressable field; raw is the value as it arrived on the wire.
type SetterFunc func(dst reflect.Value, raw string) error

// LenientBody, embedded in a Req struct, opts that route's body binding out
// of unknown-key rejection: keys no field claims are ignored rather than
// refused. Reach for it only where an older server must keep accepting a
// newer client's request; everywhere else the rejection is the feature,
// since a dropped key is how a misnamed field becomes silently-wrong
// behaviour instead of a 400.
type LenientBody struct{}

var lenientBodyType = reflect.TypeFor[LenientBody]()

// RequestParser populates a typed request from an *http.Request, binding
// fields tagged path, query, and json; per-Req plans are cached, so the
// reflect cost is paid once per (type, parser) pair. It's a concrete type
// because Parse is generic, and Go 1.27's generic methods live on concrete
// types only. The zero value is ready; set the hook fields below before
// serving traffic, since cached plans don't rebuild.
type RequestParser struct {
	// DecodeBody replaces JSON body binding wholesale. It receives the
	// request body and a pointer to the struct being bound, and is
	// responsible for populating whichever fields the wire format carries.
	// Path and query fields are already set by the time it runs. Leave it
	// nil for the built-in JSON binding, which touches only json-tagged
	// fields and reports errors per field.
	DecodeBody func(body io.Reader, dst any) error

	// Setters overrides how path and query strings are parsed into fields of
	// a given type. An entry here wins over the built-in handling for that
	// type, so this is how you bind a field type the parser would otherwise
	// reject, or change how one it already knows is read.
	Setters map[reflect.Type]SetterFunc

	cache sync.Map // reflect.Type -> *binder
}

// DefaultRequestParser is the parser dflhttp uses when no other is set on
// the Router via WithRequestParser.
var DefaultRequestParser = &RequestParser{}

// Parse builds a Req from r, binding path, query and body per Req's tags.
func (p *RequestParser) Parse[Req any](r *http.Request) (Req, error) {
	var req Req

	b, err := p.binderFor(reflect.TypeFor[Req]())
	if err != nil {
		return req, err
	}

	if err := b.bind(p, r, &req); err != nil {
		return req, err
	}

	return req, nil
}

// PrepareFor compiles and caches the binding plan for Req. adapt calls this
// at registration to surface tag errors before the first request.
func (p *RequestParser) PrepareFor[Req any]() error {
	_, err := p.binderFor(reflect.TypeFor[Req]())

	return err
}

func (p *RequestParser) binderFor(t reflect.Type) (*binder, error) {
	if cached, ok := p.cache.Load(t); ok {
		return cached.(*binder), nil
	}

	b, err := p.buildBinder(t)
	if err != nil {
		return nil, err
	}

	actual, _ := p.cache.LoadOrStore(t, b)

	return actual.(*binder), nil
}

// --- internal binder, the only place reflection lives ---

type binder struct {
	paths     []paramBind
	queries   []paramBind
	body      []bodyBind
	knownBody map[string]struct{}
	hasBody   bool
	lenient   bool
	noop      bool
}

type paramBind struct {
	key      string
	fieldIdx []int
	setter   func(reflect.Value, string) error
}

type bodyBind struct {
	key      string
	fieldIdx []int
}

var (
	emptyType    = reflect.TypeFor[Empty]()
	emptyPtrType = reflect.TypeFor[*Empty]()
)

func isEmptyType(t reflect.Type) bool {
	return t == emptyType || t == emptyPtrType
}

// buildBinder reflects on t once and returns a binder that knows where each
// field of t comes from (path, query, or body). t may be a struct, a
// pointer to a struct, Empty, or *Empty.
func (p *RequestParser) buildBinder(t reflect.Type) (*binder, error) {
	if isEmptyType(t) {
		return &binder{noop: true}, nil
	}

	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("req must be a struct, *struct, or http.Empty, got %s", t.Kind())
	}

	b := &binder{knownBody: map[string]struct{}{}}

	for f := range t.Fields() {
		if !f.IsExported() {
			continue
		}

		if f.Type == lenientBodyType {
			b.lenient = true

			continue
		}

		if pathTag := f.Tag.Get("path"); pathTag != "" {
			setter, err := p.stringSetter(f.Type)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", f.Name, err)
			}

			b.paths = append(b.paths, paramBind{key: pathTag, fieldIdx: f.Index, setter: setter})

			continue
		}

		if queryTag := f.Tag.Get("query"); queryTag != "" {
			setter, err := p.stringSetter(f.Type)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", f.Name, err)
			}

			b.queries = append(b.queries, paramBind{key: queryTag, fieldIdx: f.Index, setter: setter})

			continue
		}

		if jsonTag := f.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			key, _, _ := strings.Cut(jsonTag, ",")
			b.body = append(b.body, bodyBind{key: key, fieldIdx: f.Index})
			b.knownBody[key] = struct{}{}
			b.hasBody = true
		}
	}

	return b, nil
}

// bind populates dst (a *Req) from r. When Req is itself a pointer to a
// struct, dst is **Struct; we walk one indirection and allocate the inner
// pointer so the binder can write into a real value.
func (b *binder) bind(p *RequestParser, r *http.Request, dst any) error {
	if b.noop {
		return nil
	}

	v := reflect.ValueOf(dst).Elem()

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}

		v = v.Elem()
	}

	// Failures collect rather than short-circuit, so one response names
	// every bad input and the client fixes them in one round trip.
	var reasons []Reason

	for _, pb := range b.paths {
		val := r.PathValue(pb.key)
		if val == "" {
			continue
		}

		if err := pb.setter(v.FieldByIndex(pb.fieldIdx), val); err != nil {
			reasons = append(reasons, paramReason("path", pb.key, err))
		}
	}

	if len(b.queries) > 0 {
		query := r.URL.Query()

		for _, q := range b.queries {
			val := query.Get(q.key)
			if val == "" {
				continue
			}

			if err := q.setter(v.FieldByIndex(q.fieldIdx), val); err != nil {
				reasons = append(reasons, paramReason("query", q.key, err))
			}
		}
	}

	if b.hasBody {
		bodyReasons, err := b.bindBody(p, r, v)
		if err != nil {
			return err
		}

		reasons = append(reasons, bodyReasons...)
	}

	if len(reasons) > 0 {
		return New("invalid_request", nil).WithReasons(reasons...)
	}

	return nil
}

// bindBody binds json-tagged fields and returns one Reason per failure:
// fields that won't unmarshal, and, unless the Req embeds LenientBody, body
// keys no field claims. The error return is reserved for failures that
// preempt binding entirely: a non-JSON content type, or a custom decoder's
// own error.
func (b *binder) bindBody(p *RequestParser, r *http.Request, dst reflect.Value) ([]Reason, error) {
	// A custom decoder owns the body outright: it picks the wire format, so
	// the JSON content-type check, per-field binding, and unknown-key
	// rejection below don't apply.
	if p.DecodeBody != nil {
		return nil, p.DecodeBody(r.Body, dst.Addr().Interface())
	}

	if ct := r.Header.Get("Content-Type"); ct != "" {
		mt, _, _ := strings.Cut(ct, ";")
		if strings.TrimSpace(mt) != "application/json" {
			return nil, New("unsupported_media_type", M{
				"content_type": ct,
			})
		}
	}

	raw := map[string]json.RawMessage{}

	err := json.NewDecoder(r.Body).Decode(&raw)
	if err != nil && !errors.Is(err, io.EOF) {
		return []Reason{{Code: "malformed", Meta: M{"in": "body", "error": err.Error()}}}, nil
	}

	var reasons []Reason

	for _, fb := range b.body {
		rm, ok := raw[fb.key]
		if !ok {
			continue
		}

		if err := json.Unmarshal(rm, dst.FieldByIndex(fb.fieldIdx).Addr().Interface()); err != nil {
			reasons = append(reasons, bodyFieldReason(fb.key, err))
		}
	}

	if !b.lenient {
		reasons = append(reasons, b.unknownKeyReasons(raw)...)
	}

	return reasons, nil
}

// unknownKeyReasons returns one Reason per body key no field claims, sorted
// so the wire shape is deterministic. Dropping them instead is only safe
// when client and server schemas are guaranteed to agree, and the entire
// point of versioning an API is that they don't.
func (b *binder) unknownKeyReasons(raw map[string]json.RawMessage) []Reason {
	var unknown []string

	for key := range raw {
		if _, ok := b.knownBody[key]; !ok {
			unknown = append(unknown, key)
		}
	}

	slices.Sort(unknown)

	reasons := make([]Reason, 0, len(unknown))
	for _, key := range unknown {
		reasons = append(reasons, Reason{Code: "unknown_field", Meta: M{"in": "body", "field": key}})
	}

	return reasons
}

// paramReason is the Reason for an unparseable path or query value. field
// is always the name the caller used on the wire, never the Go field name.
func paramReason(in, field string, err error) Reason {
	return Reason{Code: "invalid", Meta: M{"in": in, "field": field, "error": err.Error()}}
}

// bodyFieldReason distinguishes a type mismatch, where the decoder can name
// what it wanted and what arrived, from any other per-field failure.
func bodyFieldReason(field string, err error) Reason {
	if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		return Reason{Code: "invalid_type", Meta: M{
			"in":       "body",
			"field":    field,
			"expected": typeErr.Type.String(),
			"got":      typeErr.Value,
		}}
	}

	return Reason{Code: "invalid", Meta: M{"in": "body", "field": field, "error": err.Error()}}
}

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

// stringSetter returns a function that parses a string into a typed
// reflect.Value. Used for path and query params, which are always strings
// on the wire. An entry in p.Setters wins; otherwise the basic kinds plus
// encoding.TextUnmarshaler are supported.
func (p *RequestParser) stringSetter(t reflect.Type) (SetterFunc, error) {
	if custom, ok := p.Setters[t]; ok {
		return custom, nil
	}

	if reflect.PointerTo(t).Implements(textUnmarshalerType) {
		return func(v reflect.Value, s string) error {
			tu, ok := reflect.TypeAssert[encoding.TextUnmarshaler](v.Addr())
			if !ok {
				return errors.New("expected TextUnmarshaler")
			}

			return tu.UnmarshalText([]byte(s))
		}, nil
	}

	switch t.Kind() {
	case reflect.String:
		return func(v reflect.Value, s string) error {
			v.SetString(s)

			return nil
		}, nil

	case reflect.Bool:
		return func(v reflect.Value, s string) error {
			x, err := strconv.ParseBool(s)
			if err != nil {
				return err
			}

			v.SetBool(x)

			return nil
		}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(v reflect.Value, s string) error {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return err
			}

			v.SetInt(n)

			return nil
		}, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(v reflect.Value, s string) error {
			n, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return err
			}

			v.SetUint(n)

			return nil
		}, nil

	case reflect.Float32, reflect.Float64:
		return func(v reflect.Value, s string) error {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return err
			}

			v.SetFloat(f)

			return nil
		}, nil

	default:
		return nil, fmt.Errorf("unsupported field type %s", t)
	}
}
