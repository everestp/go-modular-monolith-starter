package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateMigration(t *testing.T) {
	directory := t.TempDir()
	path, err := createMigration(directory, "add_audit_log")
	if err != nil {
		t.Fatalf("createMigration returned error: %v", err)
	}
	if filepath.Dir(path) != directory || !strings.HasSuffix(path, "_add_audit_log.sql") {
		t.Fatalf("migration path = %q", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration file: %v", err)
	}
	if string(content) != "-- +goose Up\n\n-- +goose Down\n" {
		t.Fatalf("unexpected migration template: %q", content)
	}
}

func TestCreateMigrationRejectsUnsafeName(t *testing.T) {
	if _, err := createMigration(t.TempDir(), "../escape"); err == nil {
		t.Fatal("createMigration accepted a path traversal name")
	}
}
