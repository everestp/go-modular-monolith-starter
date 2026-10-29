// Package logger configures structured logging for the application.
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// New builds an slog.Logger using JSON in production and text in development.
func New(level, format string) *slog.Logger {
	lvl := slog.LevelInfo
	switch strings.ToUpper(level) {
	case "DEBUG":
		lvl = slog.LevelDebug
	case "INFO":
		lvl = slog.LevelInfo
	case "WARN", "WARNING":
		lvl = slog.LevelWarn
	case "ERROR":
		lvl = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if strings.ToLower(format) == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

// Discard returns a logger that writes nowhere.
func Discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
