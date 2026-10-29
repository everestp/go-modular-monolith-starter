// Package service contains the auth use cases.
package service

// Service is the auth application service.
type Service struct{}

// New creates a new auth service.
func New() *Service { return &Service{} }

// Register performs user registration logic.
func (s *Service) Register(email, password string) (string, error) {
	_ = email
	_ = password
	return "registered", nil
}

// Login performs authentication logic.
func (s *Service) Login(email, password string) (string, error) {
	_ = email
	_ = password
	return "access-token", nil
}

// Me returns the current user representation.
func (s *Service) Me(userID string) (string, error) {
	_ = userID
	return "user", nil
}
