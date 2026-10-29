// Package main provides the database migration CLI.
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"prod-go/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 2 && args[0] == "create" {
		path, err := createMigration("migrations", args[1])
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	}
	if len(args) != 1 {
		return errors.New("usage: migrate <up|down|status> | migrate create <name>")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = database.Close() }()
	if err := database.Ping(); err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("configure migration dialect: %w", err)
	}
	goose.SetBaseFS(migrations.Files)
	switch args[0] {
	case "up":
		err = goose.Up(database, ".")
	case "down":
		err = goose.Down(database, ".")
	case "status":
		err = goose.Status(database, ".")
	default:
		return fmt.Errorf("unsupported command %q: use up, down, or status", args[0])
	}
	if err != nil {
		return fmt.Errorf("run %s migrations: %w", args[0], err)
	}
	return nil
}

func createMigration(directory, name string) (string, error) {
	if !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(name) {
		return "", errors.New("migration name must start with a lowercase letter and contain only lowercase letters, digits, or underscores")
	}
	filename := time.Now().UTC().Format("20060102150405") + "_" + name + ".sql"
	path := filepath.Join(directory, filename)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create migration file: %w", err)
	}
	content := "-- +goose Up\n\n-- +goose Down\n"
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write migration file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close migration file: %w", err)
	}
	return path, nil
}
