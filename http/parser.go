package http

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// RequestParser populates a typed request value from an *http.Request. The
// generic Parse method lets a single parser handle every handler shape in
// the router; routers that don't override use DefaultRequestParser.
//
// Custom parsers are useful when you want a binding model dflhttp doesn't
// ship out of the box (form bodies, msgpack, gRPC-style transcoding, an
// auth-context-aware Req that pulls the caller from middleware, etc).
type RequestParser interface {
	Parse[Req any](r *http.Request) (Req, error)
}

// preparable is an optional interface a RequestParser can satisfy to
// validate a Req shape at handler registration. The default parser
// implements it via PrepareFor; adapt type-asserts and calls it so tag
// errors surface eagerly rather than on the first request.
type preparable interface {
	PrepareFor[Req any]() error
}

// DefaultRequestParser is the parser dflhttp uses when no other is set on
// the Router via WithRequestParser. It binds fields by struct tag:
//
//	`path:"name"`  — pulled from r.PathValue(name)
//	`query:"name"` — pulled from r.URL.Query().Get(name)
//	`json:"name"`  — pulled from the JSON request body
//
// Per-Req binding plans are cached, so the reflect cost is paid once per
// (type, parser) pair.
var DefaultRequestParser RequestParser = &defaultRequestParser{}

type defaultRequestParser struct {
	cache sync.Map // reflect.Type -> *binder
}

func (p *defaultRequestParser) Parse[Req any](r *http.Request) (Req, error) {
	var req Req

	b, err := p.binderFor(reflect.TypeFor[Req]())
	if err != nil {
		return req, err
	}

	if err := b.bind(r, &req); err != nil {
		return req, err
	}

	return req, nil
}

// PrepareFor compiles and caches the binding plan for Req. adapt calls this
// at registration to surface tag errors before the first request.
func (p *defaultRequestParser) PrepareFor[Req any]() error {
	_, err := p.binderFor(reflect.TypeFor[Req]())

	return err
}

func (p *defaultRequestParser) binderFor(t reflect.Type) (*binder, error) {
	if cached, ok := p.cache.Load(t); ok {
		return cached.(*binder), nil
	}

	b, err := buildBinder(t)
	if err != nil {
		return nil, err
	}

	actual, _ := p.cache.LoadOrStore(t, b)

	return actual.(*binder), nil
}

// --- internal binder, the only place reflection lives ---

type binder struct {
	paths   []paramBind
	queries []paramBind
	body    []bodyBind
	hasBody bool
	noop    bool
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
func buildBinder(t reflect.Type) (*binder, error) {
	if isEmptyType(t) {
		return &binder{noop: true}, nil
	}

	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("req must be a struct, *struct, or http.Empty, got %s", t.Kind())
	}

	b := &binder{}

	for f := range t.Fields() {
		if !f.IsExported() {
			continue
		}

		if pathTag := f.Tag.Get("path"); pathTag != "" {
			setter, err := stringSetter(f.Type)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", f.Name, err)
			}

			b.paths = append(b.paths, paramBind{key: pathTag, fieldIdx: f.Index, setter: setter})

			continue
		}

		if queryTag := f.Tag.Get("query"); queryTag != "" {
			setter, err := stringSetter(f.Type)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", f.Name, err)
			}

			b.queries = append(b.queries, paramBind{key: queryTag, fieldIdx: f.Index, setter: setter})

			continue
		}

		if jsonTag := f.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			key := strings.SplitN(jsonTag, ",", 2)[0]
			b.body = append(b.body, bodyBind{key: key, fieldIdx: f.Index})
			b.hasBody = true
		}
	}

	return b, nil
}

// bind populates dst (a *Req) from r. When Req is itself a pointer to a
// struct, dst is **Struct; we walk one indirection and allocate the inner
// pointer so the binder can write into a real value.
func (b *binder) bind(r *http.Request, dst any) error {
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

	for _, p := range b.paths {
		val := r.PathValue(p.key)
		if val == "" {
			continue
		}

		if err := p.setter(v.FieldByIndex(p.fieldIdx), val); err != nil {
			return New(http.StatusBadRequest, "invalid_path_param", M{
				"param": p.key,
				"error": err.Error(),
			})
		}
	}

	for _, q := range b.queries {
		val := r.URL.Query().Get(q.key)
		if val == "" {
			continue
		}

		if err := q.setter(v.FieldByIndex(q.fieldIdx), val); err != nil {
			return New(http.StatusBadRequest, "invalid_query_param", M{
				"param": q.key,
				"error": err.Error(),
			})
		}
	}

	if b.hasBody {
		if err := b.bindBody(r, v); err != nil {
			return err
		}
	}

	return nil
}

func (b *binder) bindBody(r *http.Request, dst reflect.Value) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mt, _, _ := strings.Cut(ct, ";")
		if strings.TrimSpace(mt) != "application/json" {
			return New(http.StatusUnsupportedMediaType, "unsupported_media_type", M{
				"content_type": ct,
			})
		}
	}

	raw := map[string]json.RawMessage{}

	err := json.NewDecoder(r.Body).Decode(&raw)
	if err != nil && !errors.Is(err, io.EOF) {
		return New(http.StatusBadRequest, "invalid_body", M{"error": err.Error()})
	}

	for _, fb := range b.body {
		rm, ok := raw[fb.key]
		if !ok {
			continue
		}

		if err := json.Unmarshal(rm, dst.FieldByIndex(fb.fieldIdx).Addr().Interface()); err != nil {
			return New(http.StatusBadRequest, "invalid_body_field", M{
				"field": fb.key,
				"error": err.Error(),
			})
		}
	}

	return nil
}

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

// stringSetter returns a function that parses a string into a typed
// reflect.Value. Used for path and query params, which are always strings
// on the wire. Supports the basic kinds plus encoding.TextUnmarshaler.
func stringSetter(t reflect.Type) (func(reflect.Value, string) error, error) {
	if reflect.PointerTo(t).Implements(textUnmarshalerType) {
		return func(v reflect.Value, s string) error {
			tu, ok := v.Addr().Interface().(encoding.TextUnmarshaler)
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
