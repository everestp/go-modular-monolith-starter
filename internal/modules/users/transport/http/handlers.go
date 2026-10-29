// Package http contains user HTTP handlers.
package http

import "net/http"

// Register mounts user routes on the provided mux.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/users/me", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"user-1"}`))
	})
}
