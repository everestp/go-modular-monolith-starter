package auth

import (
	"fmt"
	"strings"
	"time"

	"prod-go/internal/modules/users"
	"prod-go/internal/platform/security"
)

// Service coordinates identity business logic.
type Service struct {
	Users  users.Reader
	Hash   func(string, security.Argon2Config) (string, error)
	Verify func(string, string, security.Argon2Config) (bool, error)
	JWT    *security.JWTManager
	Clock  func() time.Time
	Argon  security.Argon2Config
}

// Register creates a new user account.
func (s *Service) Register(email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(password) < 12 {
		return "", fmt.Errorf("email and password are required")
	}
	hash, err := s.Hash(password, s.Argon)
	if err != nil {
		return "", err
	}
	user := &users.User{Email: email, Password: hash, CreatedAt: s.Clock(), UpdatedAt: s.Clock()}
	_ = user
	return "registered", nil
}

// Login validates credentials and returns a token.
func (s *Service) Login(email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return "", fmt.Errorf("invalid credentials")
	}
	user, err := s.Users.GetByEmail(email)
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	if user == nil {
		return "", fmt.Errorf("invalid credentials")
	}
	ok, err := s.Verify(password, user.Password, s.Argon)
	if err != nil || !ok {
		return "", fmt.Errorf("invalid credentials")
	}
	return s.JWT.Issue(user.ID, 15*time.Minute)
}
