// Package app wires config, logger, database, and HTTP handlers together.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"prod-go/internal/config"
	authpostgres "prod-go/internal/modules/auth/repository/postgres"
	authservice "prod-go/internal/modules/auth/service"
	authhttp "prod-go/internal/modules/auth/transport/http"
	notespostgres "prod-go/internal/modules/notes/repository/postgres"
	notesservice "prod-go/internal/modules/notes/service"
	noteshttp "prod-go/internal/modules/notes/transport/http"
	userspostgres "prod-go/internal/modules/users/repository/postgres"
	"prod-go/internal/platform/clock"
	"prod-go/internal/platform/database"
	"prod-go/internal/platform/httpx"
	"prod-go/internal/platform/logger"
	"prod-go/internal/platform/security"
)

// App wires configuration, infrastructure, and HTTP services.
type App struct {
	cfg          *config.Config
	log          *slog.Logger
	pool         *pgxpool.Pool
	server       *http.Server
	shuttingDown *atomic.Bool
}

// New creates the application runtime.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	log := logger.New(cfg.LogLevel, cfg.LogFormat)
	pool, err := database.Open(ctx, database.Config{
		URL:          cfg.DatabaseURL,
		MinConns:     cfg.DBMinConns,
		MaxConns:     cfg.DBMaxConns,
		MaxLifetime:  cfg.DBMaxConnLifetime,
		MaxIdleTime:  cfg.DBMaxConnIdleTime,
		ConnTimeout:  cfg.DBConnTimeout,
		HealthPeriod: cfg.DBHealthCheckPeriod,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	userRepository := userspostgres.New(pool)
	authRepository := authpostgres.New(pool)
	noteRepository := notespostgres.New(pool)
	jwt := security.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience)
	authUseCases, err := authservice.New(
		userRepository,
		authRepository,
		jwt,
		security.Argon2Config{
			Memory:      cfg.Argon2Memory * 1024,
			Iterations:  cfg.Argon2Iterations,
			Parallelism: cfg.Argon2Parallelism,
			KeyLength:   cfg.Argon2KeyLen,
			Pepper:      cfg.PasswordPepper,
		},
		cfg.AccessTokenTTL,
		cfg.RefreshTokenTTL,
		clock.Real{},
	)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create auth service: %w", err)
	}
	noteUseCases := notesservice.New(noteRepository)
	metrics := httpx.NewMetrics()

	shuttingDown := &atomic.Bool{}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if shuttingDown.Load() {
			httpx.WriteProblemFor(w, r, http.StatusServiceUnavailable, "Not Ready", "Server is shutting down.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			httpx.WriteProblemFor(w, r, http.StatusServiceUnavailable, "Not Ready", "Database is unavailable.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"dev","commit":"local","build_time":"unknown"}`))
	})
	validateToken := func(token string) (string, error) { return jwt.Validate(token) }
	authhttp.Register(mux, authUseCases, validateToken)
	noteshttp.Register(mux, noteUseCases, validateToken)
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.ServeHTTP(w, r)
		stats := pool.Stat()
		_, _ = fmt.Fprintf(w,
			"db_pool_total_conns %d\ndb_pool_acquired_conns %d\ndb_pool_idle_conns %d\ndb_pool_max_conns %d\n",
			stats.TotalConns(), stats.AcquiredConns(), stats.IdleConns(), stats.MaxConns(),
		)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteProblemFor(w, r, http.StatusNotFound, "Not Found", "The requested resource was not found.")
	})

	handler := http.Handler(mux)
	handler = httpx.ProblemMethodErrors(handler)

	limiter := httpx.NewTokenBucket(cfg.RateLimitRPS, cfg.RateLimitBurst)
	authLimiter := httpx.NewTokenBucket(max(1, cfg.RateLimitRPS/2), max(1, cfg.RateLimitBurst/2))
	handler = httpx.LimitBody(1<<20, handler)
	handler = http.TimeoutHandler(handler, 30*time.Second, `{"type":"about:blank","title":"Request Timeout","status":503}`)
	handler = httpx.RateLimitByPath(limiter, authLimiter, "/api/v1/auth/", func(r *http.Request) string {
		return httpx.ClientIP(r, cfg.TrustedProxies)
	}, handler)
	handler = httpx.CORS(cfg.CORSAllowedOrigins, handler)
	handler = httpx.Instrument(metrics, handler)
	handler = httpx.SecurityHeaders(handler)
	handler = httpx.AccessLog(log, handler)
	handler = httpx.Recover(log, handler)
	handler = httpx.WithRequestID(handler)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPReadTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		MaxHeaderBytes:    1 << 20,
		BaseContext: func(net.Listener) context.Context {
			return context.Background()
		},
	}
	return &App{cfg: cfg, log: log, pool: pool, server: server, shuttingDown: shuttingDown}, nil
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
		a.shuttingDown.Store(true)
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownTimeout)
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
