package std

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

	dflhttp "github.com/duffleone/dfl/http"
)

type binder struct {
	paths   []paramBind
	queries []paramBind
	body    []bodyBind
	hasBody bool
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

// buildBinder reflects on the Req type once at registration time and produces
// a binder that knows where each field comes from (path, query, or body).
func buildBinder(t reflect.Type) (*binder, error) {
	if isEmpty(t) {
		return &binder{}, nil
	}

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("req must be a struct or dflhttp.Empty, got %s", t.Kind())
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

func (b *binder) bind(r *http.Request, dst reflect.Value) error {
	for _, p := range b.paths {
		v := r.PathValue(p.key)
		if v == "" {
			continue
		}

		if err := p.setter(dst.FieldByIndex(p.fieldIdx), v); err != nil {
			return dflhttp.New(http.StatusBadRequest, "invalid_path_param", dflhttp.M{
				"param": p.key,
				"error": err.Error(),
			})
		}
	}

	for _, q := range b.queries {
		v := r.URL.Query().Get(q.key)
		if v == "" {
			continue
		}

		if err := q.setter(dst.FieldByIndex(q.fieldIdx), v); err != nil {
			return dflhttp.New(http.StatusBadRequest, "invalid_query_param", dflhttp.M{
				"param": q.key,
				"error": err.Error(),
			})
		}
	}

	if b.hasBody {
		if err := b.bindBody(r, dst); err != nil {
			return err
		}
	}

	return nil
}

func (b *binder) bindBody(r *http.Request, dst reflect.Value) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mt, _, _ := strings.Cut(ct, ";")
		if strings.TrimSpace(mt) != "application/json" {
			return dflhttp.New(http.StatusUnsupportedMediaType, "unsupported_media_type", dflhttp.M{
				"contentType": ct,
			})
		}
	}

	raw := map[string]json.RawMessage{}

	err := json.NewDecoder(r.Body).Decode(&raw)
	if err != nil && !errors.Is(err, io.EOF) {
		return dflhttp.New(http.StatusBadRequest, "invalid_body", dflhttp.M{"error": err.Error()})
	}

	for _, fb := range b.body {
		rm, ok := raw[fb.key]
		if !ok {
			continue
		}

		if err := json.Unmarshal(rm, dst.FieldByIndex(fb.fieldIdx).Addr().Interface()); err != nil {
			return dflhttp.New(http.StatusBadRequest, "invalid_body_field", dflhttp.M{
				"field": fb.key,
				"error": err.Error(),
			})
		}
	}

	return nil
}

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

// stringSetter returns a function that parses a string into a typed
// reflect.Value. Used for path and query params, which are always strings on
// the wire. Supports the basic kinds plus encoding.TextUnmarshaler.
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
