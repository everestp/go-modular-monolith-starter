package service

import (
	"context"
	"errors"
	"testing"

	"prod-go/internal/modules/notes"
)

type noteRepository struct {
	ownerID string
}

func (r *noteRepository) Create(_ context.Context, note *notes.Note) (*notes.Note, error) {
	return note, nil
}

func (r *noteRepository) GetByID(_ context.Context, id, ownerID string) (*notes.Note, error) {
	if ownerID != r.ownerID {
		return nil, notes.ErrNotFound
	}
	return &notes.Note{ID: id, OwnerID: ownerID}, nil
}

func (r *noteRepository) List(context.Context, string, int, string) ([]notes.Note, error) {
	return nil, nil
}

func (r *noteRepository) Update(_ context.Context, note *notes.Note) (*notes.Note, error) {
	if note.OwnerID != r.ownerID {
		return nil, notes.ErrNotFound
	}
	return note, nil
}

func (r *noteRepository) Delete(_ context.Context, _, ownerID string, _ int) error {
	if ownerID != r.ownerID {
		return notes.ErrNotFound
	}
	return nil
}

func TestCreateNote(t *testing.T) {
	result, err := New(&noteRepository{ownerID: "user-1"}).Create(context.Background(), "user-1", "title", "content")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.OwnerID != "user-1" || result.Title != "title" {
		t.Fatalf("unexpected note: %+v", result)
	}
}

func TestGetDoesNotExposeAnotherOwnersNote(t *testing.T) {
	service := New(&noteRepository{ownerID: "user-1"})
	_, err := service.Get(context.Background(), "user-2", "note-1")
	if !errors.Is(err, notes.ErrNotFound) {
		t.Fatalf("Get error = %v, want %v", err, notes.ErrNotFound)
	}
}
