// Package service contains the note use cases.
package service

import (
	"context"
	"errors"
	"strings"

	"prod-go/internal/modules/notes"
)

// Service is the note application service.
type Service struct {
	repository notes.Repository
}

// New creates a note service.
func New(repository notes.Repository) *Service { return &Service{repository: repository} }

// Create persists a new note.
func (s *Service) Create(ctx context.Context, ownerID, title, content string) (*notes.Note, error) {
	title = strings.TrimSpace(title)
	if ownerID == "" || title == "" || len(title) > 200 || len(content) > 10000 {
		return nil, errors.New("owner, title, or content is invalid")
	}
	return s.repository.Create(ctx, &notes.Note{OwnerID: ownerID, Title: title, Content: content})
}

// Get returns an active note owned by the caller.
func (s *Service) Get(ctx context.Context, ownerID, id string) (*notes.Note, error) {
	return s.repository.GetByID(ctx, id, ownerID)
}

// List returns the caller's active notes.
func (s *Service) List(ctx context.Context, ownerID string, limit int, cursor string) ([]notes.Note, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("limit must be between 1 and 100")
	}
	return s.repository.List(ctx, ownerID, limit, cursor)
}

// Update replaces a note when the caller owns the current version.
func (s *Service) Update(ctx context.Context, note *notes.Note) (*notes.Note, error) {
	if note.OwnerID == "" || note.ID == "" || strings.TrimSpace(note.Title) == "" || note.Version < 1 {
		return nil, errors.New("owner, note ID, title, and version are required")
	}
	note.Title = strings.TrimSpace(note.Title)
	return s.repository.Update(ctx, note)
}

// Delete removes a note when the caller owns the current version.
func (s *Service) Delete(ctx context.Context, ownerID, id string, version int) error {
	if ownerID == "" || id == "" || version < 1 {
		return errors.New("owner, note ID, and version are required")
	}
	return s.repository.Delete(ctx, id, ownerID, version)
}
