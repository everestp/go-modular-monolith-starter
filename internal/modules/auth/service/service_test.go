package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"prod-go/internal/modules/auth/domain"
	"prod-go/internal/modules/users"
	"prod-go/internal/platform/clock"
	"prod-go/internal/platform/security"
)

type memoryUsers struct {
	byEmail map[string]*users.User
	byID    map[string]*users.User
}

func newMemoryUsers() *memoryUsers {
	return &memoryUsers{byEmail: make(map[string]*users.User), byID: make(map[string]*users.User)}
}

func (m *memoryUsers) Create(_ context.Context, user *users.User) (*users.User, error) {
	if _, exists := m.byEmail[user.Email]; exists {
		return nil, users.ErrEmailTaken
	}
	user.ID = "user-1"
	m.byEmail[user.Email] = user
	m.byID[user.ID] = user
	return user, nil
}

func (m *memoryUsers) Update(_ context.Context, user *users.User) (*users.User, error) {
	return user, nil
}

func (m *memoryUsers) GetByEmail(_ context.Context, email string) (*users.User, error) {
	user, ok := m.byEmail[email]
	if !ok {
		return nil, users.ErrNotFound
	}
	return user, nil
}

func (m *memoryUsers) GetByID(_ context.Context, id string) (*users.User, error) {
	user, ok := m.byID[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	return user, nil
}

type memoryRefreshTokens struct {
	tokens   map[string]string
	families map[string]string
	revoked  map[string]bool
}

func newMemoryRefreshTokens() *memoryRefreshTokens {
	return &memoryRefreshTokens{tokens: make(map[string]string), families: make(map[string]string), revoked: make(map[string]bool)}
}

func (m *memoryRefreshTokens) Create(_ context.Context, userID, familyID, tokenHash string, _ time.Time) error {
	m.tokens[tokenHash] = userID
	m.families[tokenHash] = familyID
	return nil
}

func (m *memoryRefreshTokens) Rotate(_ context.Context, oldHash, newHash string, _ time.Time) (string, string, error) {
	userID, exists := m.tokens[oldHash]
	if !exists {
		return "", "", domain.ErrInvalidRefreshToken
	}
	familyID := m.families[oldHash]
	if m.revoked[familyID] {
		return "", "", domain.ErrInvalidRefreshToken
	}
	if _, active := m.tokens["active:"+oldHash]; active {
		m.revoked[familyID] = true
		return "", "", domain.ErrRefreshReuse
	}
	m.tokens["active:"+oldHash] = userID
	m.tokens[newHash] = userID
	m.families[newHash] = familyID
	return userID, familyID, nil
}

func (m *memoryRefreshTokens) RevokeFamily(_ context.Context, userID, tokenHash string) error {
	if m.tokens[tokenHash] == userID {
		m.revoked[m.families[tokenHash]] = true
	}
	return nil
}

func newTestService(t *testing.T) (*Service, *memoryRefreshTokens) {
	t.Helper()
	store := newMemoryRefreshTokens()
	service, err := New(
		newMemoryUsers(), store,
		security.NewJWTManager("01234567890123456789012345678901", "issuer", "audience"),
		security.Argon2Config{Memory: 64, Iterations: 1, Parallelism: 1, KeyLength: 16},
		15*time.Minute, time.Hour, clock.NewFixed(time.Unix(100, 0)),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return service, store
}

func TestRegisterLoginRefreshAndDetectReuse(t *testing.T) {
	service, tokens := newTestService(t)
	ctx := context.Background()
	initial, user, err := service.Register(ctx, " User@example.com ", "strong-password-123")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if user.Email != "user@example.com" || initial.RefreshToken == "" {
		t.Fatalf("unexpected registration result: user=%+v session=%+v", user, initial)
	}
	if _, _, err := service.Login(ctx, "USER@example.com", "strong-password-123"); err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	rotated, err := service.Refresh(ctx, initial.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if _, err := service.Refresh(ctx, initial.RefreshToken); !errors.Is(err, domain.ErrRefreshReuse) {
		t.Fatalf("replay error = %v, want %v", err, domain.ErrRefreshReuse)
	}
	if _, err := service.Refresh(ctx, rotated.RefreshToken); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("rotated family error = %v, want %v", err, domain.ErrInvalidRefreshToken)
	}
	if _, err := service.Me(ctx, user.ID); err != nil {
		t.Fatalf("Me returned error: %v", err)
	}
	if len(tokens.revoked) != 1 {
		t.Fatalf("revoked families = %d, want 1", len(tokens.revoked))
	}
}

func TestLoginUsesGenericErrorForUnknownEmail(t *testing.T) {
	service, _ := newTestService(t)
	if _, _, err := service.Login(context.Background(), "missing@example.com", "wrong-password"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("Login error = %v, want generic invalid credentials", err)
	}
}
