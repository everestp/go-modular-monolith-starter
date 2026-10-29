//go:build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	authdomain "prod-go/internal/modules/auth/domain"
	authpostgres "prod-go/internal/modules/auth/repository/postgres"
	"prod-go/internal/modules/notes"
	notespostgres "prod-go/internal/modules/notes/repository/postgres"
	"prod-go/internal/modules/users"
	userspostgres "prod-go/internal/modules/users/repository/postgres"
	"prod-go/migrations"
)

func TestPostgresRepositories(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}
	ctx := context.Background()
	connection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open migration connection: %v", err)
	}
	defer func() { _ = connection.Close() }()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set Goose dialect: %v", err)
	}
	goose.SetBaseFS(migrations.Files)
	if err := goose.Up(connection, "."); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	userRepository := userspostgres.New(pool)
	email := fmt.Sprintf("integration-%d@example.com", time.Now().UnixNano())
	user, err := userRepository.Create(ctx, &users.User{Email: email, Password: "test-hash"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
			t.Errorf("clean integration user: %v", err)
		}
	}()
	if _, err := userRepository.GetByEmail(ctx, email); err != nil {
		t.Fatalf("get user by email: %v", err)
	}

	noteRepository := notespostgres.New(pool)
	first, err := noteRepository.Create(ctx, &notes.Note{OwnerID: user.ID, Title: "one", Content: "first"})
	if err != nil {
		t.Fatalf("create first note: %v", err)
	}
	if _, err := noteRepository.Create(ctx, &notes.Note{OwnerID: user.ID, Title: "two", Content: "second"}); err != nil {
		t.Fatalf("create second note: %v", err)
	}
	if _, err := noteRepository.GetByID(ctx, first.ID, "00000000-0000-4000-8000-000000000001"); !errors.Is(err, notes.ErrNotFound) {
		t.Fatalf("cross-owner read error = %v, want not found", err)
	}
	page, err := noteRepository.List(ctx, user.ID, 1, "")
	if err != nil || len(page) != 1 {
		t.Fatalf("first note page = %d rows, error %v", len(page), err)
	}
	cursor := base64.RawURLEncoding.EncodeToString([]byte(page[0].CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + page[0].ID))
	nextPage, err := noteRepository.List(ctx, user.ID, 1, cursor)
	if err != nil || len(nextPage) != 1 || nextPage[0].ID == page[0].ID {
		t.Fatalf("next note page = %#v, error %v", nextPage, err)
	}

	first.Title = "updated"
	updated, err := noteRepository.Update(ctx, first)
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if updated.Version != first.Version+1 {
		t.Fatalf("update note version = %d, want %d", updated.Version, first.Version+1)
	}
	if _, err := noteRepository.Update(ctx, first); !errors.Is(err, notes.ErrConflict) {
		t.Fatalf("stale update error = %v, want conflict", err)
	}

	tokenRepository := authpostgres.New(pool)
	familyID := "00000000-0000-4000-8000-000000000002"
	if err := tokenRepository.Create(ctx, user.ID, familyID, "old-token-hash", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create refresh token: %v", err)
	}
	if _, _, err := tokenRepository.Rotate(ctx, "old-token-hash", "new-token-hash", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("rotate refresh token: %v", err)
	}
	if _, _, err := tokenRepository.Rotate(ctx, "old-token-hash", "replay-token-hash", time.Now().Add(time.Hour)); !errors.Is(err, authdomain.ErrRefreshReuse) {
		t.Fatalf("replay error = %v, want reuse detection", err)
	}
	if _, _, err := tokenRepository.Rotate(ctx, "new-token-hash", "third-token-hash", time.Now().Add(time.Hour)); !errors.Is(err, authdomain.ErrInvalidRefreshToken) {
		t.Fatalf("family token after replay = %v, want invalid token", err)
	}
}
