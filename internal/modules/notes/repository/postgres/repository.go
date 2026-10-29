// Package postgres contains the note persistence implementation.
package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"prod-go/internal/modules/notes"
	"prod-go/internal/platform/database"

	"github.com/jackc/pgx/v5"
)

// Repository persists notes.
type Repository struct {
	db database.DBTX
}

// New creates a note repository.
func New(db database.DBTX) *Repository { return &Repository{db: db} }

// Create inserts a note owned by the authenticated user.
func (r *Repository) Create(ctx context.Context, note *notes.Note) (*notes.Note, error) {
	const query = `
		INSERT INTO notes (owner_id, title, content)
		VALUES ($1, $2, $3)
		RETURNING id::text, owner_id::text, title, content, created_at, updated_at, version`

	created := new(notes.Note)
	err := database.Executor(ctx, r.db).QueryRow(ctx, query, note.OwnerID, note.Title, note.Content).Scan(
		&created.ID, &created.OwnerID, &created.Title, &created.Content,
		&created.CreatedAt, &created.UpdatedAt, &created.Version,
	)
	if err != nil {
		return nil, fmt.Errorf("insert note: %w", err)
	}
	return created, nil
}

// GetByID returns a note only when it belongs to the requested owner.
func (r *Repository) GetByID(ctx context.Context, id, ownerID string) (*notes.Note, error) {
	const query = `
		SELECT id::text, owner_id::text, title, content, created_at, updated_at, version
		FROM notes WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`

	note := new(notes.Note)
	err := database.Executor(ctx, r.db).QueryRow(ctx, query, id, ownerID).Scan(
		&note.ID, &note.OwnerID, &note.Title, &note.Content,
		&note.CreatedAt, &note.UpdatedAt, &note.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notes.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get note: %w", err)
	}
	return note, nil
}

// List returns a page of active notes ordered by creation time and ID.
func (r *Repository) List(ctx context.Context, ownerID string, limit int, cursor string) ([]notes.Note, error) {
	var cursorTime *time.Time
	var cursorID *string
	if cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, fmt.Errorf("decode note cursor: %w", err)
		}
		parts := strings.SplitN(string(decoded), "|", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid note cursor")
		}
		parsedTime, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return nil, fmt.Errorf("parse note cursor time: %w", err)
		}
		cursorTime = &parsedTime
		cursorID = &parts[1]
	}
	const query = `
		SELECT id::text, owner_id::text, title, content, created_at, updated_at, version
		FROM notes
		WHERE owner_id = $1 AND deleted_at IS NULL
		  AND ($2::timestamptz IS NULL OR (created_at, id) < ($2, $3::uuid))
		ORDER BY created_at DESC, id DESC
		LIMIT $4`
	rows, err := database.Executor(ctx, r.db).Query(ctx, query, ownerID, cursorTime, cursorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()
	result := make([]notes.Note, 0, limit)
	for rows.Next() {
		var note notes.Note
		if err := rows.Scan(&note.ID, &note.OwnerID, &note.Title, &note.Content, &note.CreatedAt, &note.UpdatedAt, &note.Version); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		result = append(result, note)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notes: %w", err)
	}
	return result, nil
}

// Update replaces note content only when the supplied version is current.
func (r *Repository) Update(ctx context.Context, note *notes.Note) (*notes.Note, error) {
	const query = `
		UPDATE notes SET title = $3, content = $4, updated_at = NOW(), version = version + 1
		WHERE id = $1 AND owner_id = $2 AND version = $5 AND deleted_at IS NULL
		RETURNING id::text, owner_id::text, title, content, created_at, updated_at, version`

	updated := new(notes.Note)
	err := database.Executor(ctx, r.db).QueryRow(ctx, query, note.ID, note.OwnerID, note.Title, note.Content, note.Version).Scan(
		&updated.ID, &updated.OwnerID, &updated.Title, &updated.Content,
		&updated.CreatedAt, &updated.UpdatedAt, &updated.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, lookupErr := r.GetByID(ctx, note.ID, note.OwnerID); lookupErr != nil {
			return nil, notes.ErrNotFound
		}
		return nil, notes.ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf("update note: %w", err)
	}
	return updated, nil
}

// Delete soft-deletes a note only when the supplied version is current.
func (r *Repository) Delete(ctx context.Context, id, ownerID string, version int) error {
	const query = `
		UPDATE notes SET deleted_at = NOW(), updated_at = NOW(), version = version + 1
		WHERE id = $1 AND owner_id = $2 AND version = $3 AND deleted_at IS NULL`
	result, err := database.Executor(ctx, r.db).Exec(ctx, query, id, ownerID, version)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	if result.RowsAffected() == 0 {
		if _, lookupErr := r.GetByID(ctx, id, ownerID); lookupErr != nil {
			return notes.ErrNotFound
		}
		return notes.ErrConflict
	}
	return nil
}
