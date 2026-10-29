// Package httpx provides reusable HTTP helpers and middleware primitives.
package httpx

import (
	"encoding/json"
	"net/http"
)

// DecodeJSON reads a JSON request body into a target value.
func DecodeJSON(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
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
	_, _ = w.Write([]byte(`{"title":"` + title + `","detail":"` + detail + `"}`))
}
