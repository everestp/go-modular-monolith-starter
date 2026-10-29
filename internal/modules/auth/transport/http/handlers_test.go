package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterRoute(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusCreated)
	}
}
