package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"prod-go/internal/modules/auth/domain"
	"prod-go/internal/modules/users"
)

type authServiceFake struct {
	registerErr error
}

func (s authServiceFake) Register(context.Context, string, string) (*domain.Session, *users.User, error) {
	if s.registerErr != nil {
		return nil, nil, s.registerErr
	}
	return &domain.Session{AccessToken: "access", RefreshToken: "refresh"}, &users.User{ID: "user-1", Email: "user@example.com"}, nil
}

func (authServiceFake) Login(context.Context, string, string) (*domain.Session, *users.User, error) {
	return &domain.Session{AccessToken: "access", RefreshToken: "refresh"}, &users.User{ID: "user-1", Email: "user@example.com"}, nil
}

func (authServiceFake) Refresh(context.Context, string) (*domain.Session, error) {
	return &domain.Session{AccessToken: "access-2", RefreshToken: "refresh-2"}, nil
}

func (authServiceFake) Logout(context.Context, string, string) error { return nil }

func (authServiceFake) Me(context.Context, string) (*users.User, error) {
	return &users.User{ID: "user-1", Email: "user@example.com"}, nil
}

func TestRegisterRouteReturnsSession(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, authServiceFake{}, func(token string) (string, error) { return token, nil })
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"user@example.com","password":"strong-password"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"refresh_token":"refresh"`) {
		t.Fatalf("registration response = %d %s", response.Code, response.Body.String())
	}
}

func TestRegisterRejectsWrongContentTypeAndBadJSON(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, authServiceFake{}, func(token string) (string, error) { return token, nil })
	for _, test := range []struct {
		contentType string
		body        string
		wantStatus  int
	}{
		{contentType: "text/plain", body: "{}", wantStatus: http.StatusUnsupportedMediaType},
		{contentType: "application/json", body: `{"email":"user@example.com","password":"x","extra":1}`, wantStatus: http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(test.body))
		req.Header.Set("Content-Type", test.contentType)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		if response.Code != test.wantStatus {
			t.Errorf("status = %d, want %d", response.Code, test.wantStatus)
		}
	}
}

func TestRegisterMapsDuplicateEmail(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, authServiceFake{registerErr: errors.Join(errors.New("register user"), users.ErrEmailTaken)}, func(token string) (string, error) { return token, nil })
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"user@example.com","password":"strong-password"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
}

func TestProtectedMeRequiresToken(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, authServiceFake{}, func(string) (string, error) { return "", errors.New("bad token") })
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
