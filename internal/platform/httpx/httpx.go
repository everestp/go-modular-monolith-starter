// Package httpx provides reusable HTTP helpers and middleware primitives.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxJSONBody = 1 << 20

// DecodeJSON reads a JSON request body into a target value.
func DecodeJSON(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxJSONBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain a single JSON value")
		}
		return err
	}
	return nil
}

// WriteJSON writes a JSON success payload.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// WriteProblem writes a problem details response.
func WriteProblem(w http.ResponseWriter, status int, title string, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "about:blank",
		"title":  title,
		"status": status,
		"detail": detail,
	})
}
