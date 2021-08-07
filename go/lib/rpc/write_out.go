package rpc

import (
	"encoding/json"
	"net/http"
)

func WriteOut(w http.ResponseWriter, res interface{}) error {
	if res == nil {
		w.WriteHeader(204)
		return nil
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err := json.NewEncoder(w).Encode(res); err != nil {
		return err
	}

	return nil
}
