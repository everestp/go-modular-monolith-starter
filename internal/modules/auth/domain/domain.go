// Package domain defines the auth data model.
package domain

import "errors"

var (
	// ErrInvalidCredentials is returned for any login credential failure.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidRefreshToken indicates that a refresh token is expired or unknown.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	// ErrRefreshReuse indicates replay of a previously rotated refresh token.
	ErrRefreshReuse = errors.New("refresh token reuse detected")
)

// Credentials is the input for login or registration.
type Credentials struct {
	Email    string
	Password string
}

// Session represents the token pair returned to the client.
type Session struct {
	AccessToken  string
	RefreshToken string
}
