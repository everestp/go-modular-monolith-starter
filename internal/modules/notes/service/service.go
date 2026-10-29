// Package service contains the note use cases.
package service

// Service is the note application service.
type Service struct{}

// New creates a note service.
func New() *Service { return &Service{} }

// Create persists a new note.
func (s *Service) Create(title, content string) (string, error) {
	_ = title
	_ = content
	return "created", nil
}
