// Package postgres contains the auth persistence implementation.
package postgres

// Repository persists auth-related records like refresh tokens.
type Repository struct{}

// New creates an auth repository.
func New() *Repository { return &Repository{} }
