package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"prod-go/internal/config"
	"prod-go/internal/platform/database"
	"prod-go/internal/platform/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

// App wires configuration, infrastructure, and HTTP services.
type App struct {
	cfg    *config.Config
	log    *slog.Logger
	pool   *pgxpool.Pool
	server *http.Server
}

// New creates the application runtime.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log := logger.New(cfg.LogLevel, cfg.LogFormat)
	pool, err := database.Open(ctx, database.Config{
		URL:         cfg.DatabaseURL,
		MinConns:    cfg.DBMinConns,
		MaxConns:    cfg.DBMaxConns,
		MaxLifetime: cfg.DBMaxConnLifetime,
		ConnTimeout: cfg.DBConnTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, `{"status":"not-ready"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"dev","commit":"local","build_time":"unknown"}`))
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.HTTPReadTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
	return &App{cfg: cfg, log: log, pool: pool, server: server}, nil
}

// Run starts the HTTP server and watches for termination signals.
func (a *App) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case err := <-serveErr:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(ctx, a.cfg.ShutdownTimeout)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}

// Close tears down shared infrastructure.
func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}
