package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotesRoute(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notes", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}
