package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/appdb?sslmode=disable")
	t.Setenv("JWT_SECRET", "abcdefghijklmnopqrstuvwx1234567890")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("CORSAllowedOrigins len = %d, want 2", len(cfg.CORSAllowedOrigins))
	}
	if cfg.JWTSecret == "" {
		t.Fatal("JWTSecret should not be empty")
	}
	_ = os.Getenv("DATABASE_URL")
}
