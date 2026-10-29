// Package http contains auth HTTP handlers.
package http

import "net/http"

// Register mounts auth routes on the provided mux.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/register", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"registered"}`))
	})
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}
