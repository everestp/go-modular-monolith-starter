// Package postgres contains the user persistence implementation.
package postgres

// Repository persists user records.
type Repository struct{}

// New creates a user repository.
func New() *Repository { return &Repository{} }
