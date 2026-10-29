package users

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound indicates that no user matches the requested identifier.
	ErrNotFound = errors.New("user not found")
	// ErrEmailTaken indicates that a user already owns the email address.
	ErrEmailTaken = errors.New("email already registered")
)

// User is the domain entity for the application identity.
type User struct {
	ID        string
	Email     string
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Reader exposes read access to users.
type Reader interface {
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
}

// Writer exposes user persistence operations.
type Writer interface {
	Create(ctx context.Context, user *User) (*User, error)
	Update(ctx context.Context, user *User) (*User, error)
}
