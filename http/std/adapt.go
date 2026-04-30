package std

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	dflhttp "github.com/duffleone/dfl/http"
)

var (
	contextType  = reflect.TypeFor[context.Context]()
	errorType    = reflect.TypeFor[error]()
	emptyType    = reflect.TypeFor[dflhttp.Empty]()
	emptyPtrType = reflect.TypeFor[*dflhttp.Empty]()
)

func isEmpty(t reflect.Type) bool {
	return t == emptyType || t == emptyPtrType
}

// adapt validates handler's shape and returns a HandlerFunc that does
// request binding, calls handler via reflection, and writes the response.
// handler must be func(context.Context, Req) (Resp, error).
func adapt(handler any) (dflhttp.HandlerFunc, error) {
	v := reflect.ValueOf(handler)
	if v.Kind() != reflect.Func {
		return nil, errors.New("handler must be a function")
	}

	t := v.Type()
	if t.NumIn() != 2 {
		return nil, errors.New("handler must take (context.Context, Req)")
	}

	if t.NumOut() != 2 {
		return nil, errors.New("handler must return (Resp, error)")
	}

	if t.In(0) != contextType {
		return nil, errors.New("first arg must be context.Context")
	}

	if t.Out(1) != errorType {
		return nil, errors.New("second return must be error")
	}

	reqType := t.In(1)
	respType := t.Out(0)

	b, err := buildBinder(reqType)
	if err != nil {
		return nil, err
	}

	isEmptyResp := isEmpty(respType)

	return func(w http.ResponseWriter, r *http.Request) error {
		reqVal := reflect.New(reqType).Elem()

		if err := b.bind(r, reqVal); err != nil {
			return err
		}

		results := v.Call([]reflect.Value{
			reflect.ValueOf(r.Context()),
			reqVal,
		})

		if errVal := results[1]; !errVal.IsNil() {
			return errVal.Interface().(error)
		}

		if isEmptyResp {
			w.WriteHeader(http.StatusNoContent)

			return nil
		}

		w.Header().Set("Content-Type", "application/json")

		return json.NewEncoder(w).Encode(results[0].Interface())
	}, nil
}
