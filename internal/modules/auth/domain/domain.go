// Package domain defines the auth data model.
package domain

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
