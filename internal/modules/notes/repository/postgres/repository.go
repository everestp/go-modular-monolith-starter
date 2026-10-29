// Package postgres contains the note persistence implementation.
package postgres

// Repository persists notes.
type Repository struct{}

// New creates a note repository.
func New() *Repository { return &Repository{} }
