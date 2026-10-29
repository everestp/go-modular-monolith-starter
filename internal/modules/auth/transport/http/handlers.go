// Package http contains auth HTTP handlers.
package http

import (
	"context"
	"errors"
	"mime"
	"net/http"

	"prod-go/internal/modules/auth/domain"
	"prod-go/internal/modules/users"
	"prod-go/internal/platform/httpx"
)

// AuthService lists the use cases required by the auth transport layer.
type AuthService interface {
	Register(context.Context, string, string) (*domain.Session, *users.User, error)
	Login(context.Context, string, string) (*domain.Session, *users.User, error)
	Refresh(context.Context, string) (*domain.Session, error)
	Logout(context.Context, string, string) error
	Me(context.Context, string) (*users.User, error)
}

// Register mounts auth routes on the provided mux.
func Register(mux *http.ServeMux, service AuthService, validateToken func(string) (string, error)) {
	handler := &handler{service: service}
	mux.HandleFunc("POST /api/v1/auth/register", handler.register)
	mux.HandleFunc("POST /api/v1/auth/login", handler.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", handler.refresh)
	mux.Handle("POST /api/v1/auth/logout", httpx.Authenticate(validateToken, http.HandlerFunc(handler.logout)))
	mux.Handle("GET /api/v1/auth/me", httpx.Authenticate(validateToken, http.HandlerFunc(handler.me)))
}

type handler struct {
	service AuthService
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type sessionResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decodeJSON(w, r, &input) {
		return
	}
	session, user, err := h.service.Register(r.Context(), input.Email, input.Password)
	if err != nil {
		if errors.Is(err, users.ErrEmailTaken) {
			httpx.WriteProblemFor(w, r, http.StatusConflict, "Conflict", "An account with that email already exists.")
			return
		}
		if err.Error() == "email or password does not meet requirements" {
			httpx.WriteProblemFor(w, r, http.StatusUnprocessableEntity, "Unprocessable Content", err.Error())
			return
		}
		httpx.WriteProblemFor(w, r, http.StatusInternalServerError, "Internal Server Error", "Unable to register account.")
		return
	}
	writeSession(w, http.StatusCreated, session, user)
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decodeJSON(w, r, &input) {
		return
	}
	session, user, err := h.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		httpx.WriteProblemFor(w, r, http.StatusUnauthorized, "Unauthorized", "Email or password is incorrect.")
		return
	}
	writeSession(w, http.StatusOK, session, user)
}

func (h *handler) refresh(w http.ResponseWriter, r *http.Request) {
	var input refreshRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	session, err := h.service.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		httpx.WriteProblemFor(w, r, http.StatusUnauthorized, "Unauthorized", "Refresh token is invalid.")
		return
	}
	_ = httpx.WriteJSON(w, http.StatusOK, sessionResponse{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken, TokenType: "Bearer"})
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	var input refreshRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := h.service.Logout(r.Context(), httpx.AuthenticatedUserID(r.Context()), input.RefreshToken); err != nil {
		httpx.WriteProblemFor(w, r, http.StatusInternalServerError, "Internal Server Error", "Unable to log out.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.Me(r.Context(), httpx.AuthenticatedUserID(r.Context()))
	if err != nil {
		httpx.WriteProblemFor(w, r, http.StatusNotFound, "Not Found", "User not found.")
		return
	}
	_ = httpx.WriteJSON(w, http.StatusOK, userResponse{ID: user.ID, Email: user.Email})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		httpx.WriteProblemFor(w, r, http.StatusUnsupportedMediaType, "Unsupported Media Type", "Content-Type must be application/json.")
		return false
	}
	if err := httpx.DecodeJSON(r, target); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			httpx.WriteProblemFor(w, r, http.StatusRequestEntityTooLarge, "Content Too Large", "Request body exceeds the allowed size.")
			return false
		}
		httpx.WriteProblemFor(w, r, http.StatusBadRequest, "Bad Request", "Request body must be valid JSON with no unknown fields.")
		return false
	}
	return true
}

func writeSession(w http.ResponseWriter, status int, session *domain.Session, user *users.User) {
	w.Header().Set("Location", "/api/v1/auth/me")
	_ = httpx.WriteJSON(w, status, struct {
		sessionResponse
		User userResponse `json:"user"`
	}{
		sessionResponse: sessionResponse{AccessToken: session.AccessToken, RefreshToken: session.RefreshToken, TokenType: "Bearer"},
		User:            userResponse{ID: user.ID, Email: user.Email},
	})
}
