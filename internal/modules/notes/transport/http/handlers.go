// Package http contains note HTTP handlers.
package http

import "net/http"

// Register mounts note routes on the provided mux.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/notes", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}
