// Package service contains the user use case layer.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"prod-go/internal/modules/users"
)

// Service is the user application service.
type Service struct {
	users users.Writer
}

// New creates a user service.
func New(repository users.Writer) *Service { return &Service{users: repository} }

// Create stores a new user.
func (s *Service) Create(ctx context.Context, user *users.User) (*users.User, error) {
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	if user.Email == "" || !strings.Contains(user.Email, "@") || user.Password == "" {
		return nil, errors.New("valid email and password hash are required")
	}
	created, err := s.users.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return created, nil
}
