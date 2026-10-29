// Package service contains the auth use cases.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"prod-go/internal/modules/auth/domain"
	"prod-go/internal/modules/users"
	"prod-go/internal/platform/clock"
	"prod-go/internal/platform/security"
)

type userStore interface {
	users.Reader
	users.Writer
}

type refreshStore interface {
	Create(ctx context.Context, userID, familyID, tokenHash string, expiresAt time.Time) error
	Rotate(ctx context.Context, oldHash, newHash string, expiresAt time.Time) (string, string, error)
	RevokeFamily(ctx context.Context, userID, tokenHash string) error
}

// Service implements registration, login, and refresh-token lifecycle use cases.
type Service struct {
	users          userStore
	refresh        refreshStore
	jwt            *security.JWTManager
	hashPassword   func(string, security.Argon2Config) (string, error)
	verifyPassword func(string, string, security.Argon2Config) (bool, error)
	argon          security.Argon2Config
	accessTTL      time.Duration
	refreshTTL     time.Duration
	dummyHash      string
	clock          clock.Clock
}

// New creates an auth service and prepares its timing-safe missing-user hash.
func New(
	userStore userStore,
	refreshStore refreshStore,
	jwt *security.JWTManager,
	argon security.Argon2Config,
	accessTTL, refreshTTL time.Duration,
	now clock.Clock,
) (*Service, error) {
	dummyHash, err := security.HashPassword("nonexistent-account-password", argon)
	if err != nil {
		return nil, fmt.Errorf("prepare timing-safe password hash: %w", err)
	}
	if now == nil {
		now = clock.Real{}
	}
	return &Service{
		users: userStore, refresh: refreshStore, jwt: jwt,
		hashPassword: security.HashPassword, verifyPassword: security.VerifyPassword,
		argon: argon, accessTTL: accessTTL, refreshTTL: refreshTTL,
		dummyHash: dummyHash, clock: now,
	}, nil
}

// Register creates an account and issues its initial token pair.
func (s *Service) Register(ctx context.Context, email, password string) (*domain.Session, *users.User, error) {
	email = normalizeEmail(email)
	if !validEmail(email) || len(password) < 12 || len(password) > 128 {
		return nil, nil, errors.New("email or password does not meet requirements")
	}
	hash, err := s.hashPassword(password, s.argon)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.users.Create(ctx, &users.User{Email: email, Password: hash})
	if err != nil {
		return nil, nil, fmt.Errorf("register user: %w", err)
	}
	session, err := s.issueSession(ctx, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("issue session: %w", err)
	}
	return session, user, nil
}

// Login returns a generic error for unknown users and incorrect passwords.
func (s *Service) Login(ctx context.Context, email, password string) (*domain.Session, *users.User, error) {
	user, err := s.users.GetByEmail(ctx, normalizeEmail(email))
	if err != nil || user == nil {
		_, _ = s.verifyPassword(password, s.dummyHash, s.argon)
		return nil, nil, domain.ErrInvalidCredentials
	}
	valid, err := s.verifyPassword(password, user.Password, s.argon)
	if err != nil || !valid {
		return nil, nil, domain.ErrInvalidCredentials
	}
	session, err := s.issueSession(ctx, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("issue session: %w", err)
	}
	return session, user, nil
}

// Refresh rotates a refresh token and revokes its family when replay is detected.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*domain.Session, error) {
	if refreshToken == "" {
		return nil, domain.ErrInvalidRefreshToken
	}
	newToken, err := newOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	userID, _, err := s.refresh.Rotate(ctx, hashToken(refreshToken), hashToken(newToken), s.clock.Now().Add(s.refreshTTL))
	if err != nil {
		return nil, err
	}
	accessToken, err := s.jwt.Issue(userID, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}
	return &domain.Session{AccessToken: accessToken, RefreshToken: newToken}, nil
}

// Logout revokes the refresh-token family associated with the caller's token.
func (s *Service) Logout(ctx context.Context, userID, refreshToken string) error {
	if userID == "" || refreshToken == "" {
		return nil
	}
	if err := s.refresh.RevokeFamily(ctx, userID, hashToken(refreshToken)); err != nil {
		return fmt.Errorf("revoke refresh token family: %w", err)
	}
	return nil
}

// Me returns the current user without exposing password data.
func (s *Service) Me(ctx context.Context, userID string) (*users.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	user.Password = ""
	return user, nil
}

func (s *Service) issueSession(ctx context.Context, userID string) (*domain.Session, error) {
	refreshToken, err := newOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	familyID, err := newUUID()
	if err != nil {
		return nil, fmt.Errorf("generate token family: %w", err)
	}
	if err := s.refresh.Create(ctx, userID, familyID, hashToken(refreshToken), s.clock.Now().Add(s.refreshTTL)); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}
	accessToken, err := s.jwt.Issue(userID, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}
	return &domain.Session{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func validEmail(email string) bool {
	parts := strings.Split(email, "@")
	return len(parts) == 2 && parts[0] != "" && parts[1] != "" && len(email) <= 320
}

func newOpaqueToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}
