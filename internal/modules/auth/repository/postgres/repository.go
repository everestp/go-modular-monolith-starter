// Package postgres contains the auth persistence implementation.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"prod-go/internal/modules/auth/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists auth-related records like refresh tokens.
type Repository struct {
	pool *pgxpool.Pool
}

// New creates an auth repository.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Create stores the hash of a refresh token in a new family.
func (r *Repository) Create(ctx context.Context, userID, familyID, tokenHash string, expiresAt time.Time) error {
	const query = `INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`
	if _, err := r.pool.Exec(ctx, query, userID, familyID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// Rotate atomically consumes a refresh token and inserts its replacement.
func (r *Repository) Rotate(ctx context.Context, oldHash, newHash string, expiresAt time.Time) (string, string, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", "", fmt.Errorf("begin refresh rotation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const selectQuery = `
		SELECT user_id::text, family_id::text, expires_at, revoked_at, replaced_by_hash
		FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`
	var userID, familyID string
	var expires time.Time
	var revoked pgtype.Timestamptz
	var replacedBy pgtype.Text
	err = tx.QueryRow(ctx, selectQuery, oldHash).Scan(&userID, &familyID, &expires, &revoked, &replacedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", domain.ErrInvalidRefreshToken
	}
	if err != nil {
		return "", "", fmt.Errorf("read refresh token: %w", err)
	}
	if revoked.Valid {
		if !replacedBy.Valid {
			return "", "", domain.ErrInvalidRefreshToken
		}
		if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, NOW()) WHERE family_id = $1`, familyID); err != nil {
			return "", "", fmt.Errorf("revoke reused token family: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return "", "", fmt.Errorf("commit reused token revocation: %w", err)
		}
		return "", "", domain.ErrRefreshReuse
	}
	if !expires.After(time.Now()) {
		return "", "", domain.ErrInvalidRefreshToken
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = NOW(), replaced_by_hash = $2 WHERE token_hash = $1`, oldHash, newHash); err != nil {
		return "", "", fmt.Errorf("consume refresh token: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`, userID, familyID, newHash, expiresAt); err != nil {
		return "", "", fmt.Errorf("store rotated refresh token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("commit refresh rotation: %w", err)
	}
	return userID, familyID, nil
}

// RevokeFamily revokes the family containing the caller's refresh token.
func (r *Repository) RevokeFamily(ctx context.Context, userID, tokenHash string) error {
	const query = `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE family_id = (
			SELECT family_id FROM refresh_tokens WHERE user_id = $1 AND token_hash = $2
		)`
	if _, err := r.pool.Exec(ctx, query, userID, tokenHash); err != nil {
		return fmt.Errorf("revoke refresh family: %w", err)
	}
	return nil
}
