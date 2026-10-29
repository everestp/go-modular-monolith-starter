// Package service contains the user use case layer.
package service

// Service is the user application service.
type Service struct{}

// New creates a user service.
func New() *Service { return &Service{} }

// Create stores a new user.
func (s *Service) Create(email string) (string, error) {
	_ = email
	return "created", nil
}
