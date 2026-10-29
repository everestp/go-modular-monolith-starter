package notes

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound indicates that the note does not exist for the requested owner.
	ErrNotFound = errors.New("note not found")
	// ErrConflict indicates that the note changed since it was read.
	ErrConflict = errors.New("note version conflict")
)

// Note is the resource managed by the API.
type Note struct {
	ID        string
	OwnerID   string
	Title     string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int
}

// Repository describes persistence for notes.
type Repository interface {
	Create(ctx context.Context, note *Note) (*Note, error)
	GetByID(ctx context.Context, id, ownerID string) (*Note, error)
	List(ctx context.Context, ownerID string, limit int, cursor string) ([]Note, error)
	Update(ctx context.Context, note *Note) (*Note, error)
	Delete(ctx context.Context, id, ownerID string, version int) error
}
