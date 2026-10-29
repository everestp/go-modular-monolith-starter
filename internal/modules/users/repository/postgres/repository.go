// Package postgres contains the user persistence implementation.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"prod-go/internal/modules/users"
	"prod-go/internal/platform/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Repository persists user records.
type Repository struct {
	db database.DBTX
}

// New creates a user repository.
func New(db database.DBTX) *Repository { return &Repository{db: db} }

// Create inserts a user and returns the persisted record.
func (r *Repository) Create(ctx context.Context, user *users.User) (*users.User, error) {
	const query = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id::text, email, password_hash, created_at, updated_at`

	created := new(users.User)
	err := database.Executor(ctx, r.db).QueryRow(ctx, query, user.Email, user.Password).Scan(
		&created.ID, &created.Email, &created.Password, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, users.ErrEmailTaken
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return created, nil
}

// GetByEmail looks up a user by its normalized email address.
func (r *Repository) GetByEmail(ctx context.Context, email string) (*users.User, error) {
	return r.get(ctx, `SELECT id::text, email, password_hash, created_at, updated_at FROM users WHERE email = $1`, email)
}

// GetByID looks up a user by its identifier.
func (r *Repository) GetByID(ctx context.Context, id string) (*users.User, error) {
	return r.get(ctx, `SELECT id::text, email, password_hash, created_at, updated_at FROM users WHERE id = $1`, id)
}

func (r *Repository) get(ctx context.Context, query, value string) (*users.User, error) {
	user := new(users.User)
	err := database.Executor(ctx, r.db).QueryRow(ctx, query, value).Scan(
		&user.ID, &user.Email, &user.Password, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, users.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// Update changes a user's mutable fields.
func (r *Repository) Update(ctx context.Context, user *users.User) (*users.User, error) {
	const query = `
		UPDATE users SET email = $2, password_hash = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id::text, email, password_hash, created_at, updated_at`

	updated := new(users.User)
	err := database.Executor(ctx, r.db).QueryRow(ctx, query, user.ID, user.Email, user.Password).Scan(
		&updated.ID, &updated.Email, &updated.Password, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, users.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return updated, nil
}
