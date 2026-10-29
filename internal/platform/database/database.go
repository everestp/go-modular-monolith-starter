// Package database provides PostgreSQL connection pooling helpers.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txContextKey struct{}

// DBTX is the query surface shared by pgx pools and transactions.
type DBTX interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Config wraps the pgx pool configuration.
type Config struct {
	URL          string
	MinConns     int32
	MaxConns     int32
	MaxLifetime  time.Duration
	MaxIdleTime  time.Duration
	ConnTimeout  time.Duration
	HealthPeriod time.Duration
}

// Open creates and returns a PostgreSQL connection pool.
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MaxConnLifetime = cfg.MaxLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthPeriod
	if poolCfg.HealthCheckPeriod <= 0 {
		poolCfg.HealthCheckPeriod = 30 * time.Second
	}
	poolCfg.ConnConfig.ConnectTimeout = cfg.ConnTimeout
	poolCfg.ConnConfig.RuntimeParams = map[string]string{
		"application_name":  "prod-go",
		"statement_timeout": "5000",
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// TxManager is a simple abstraction for transaction management.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

// PGXTxManager executes callbacks in a pgx transaction and exposes it through context.
type PGXTxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a transaction manager for a pool.
func NewTxManager(pool *pgxpool.Pool) *PGXTxManager { return &PGXTxManager{pool: pool} }

// WithinTx commits when fn succeeds and rolls back when it fails.
func (m *PGXTxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(context.WithValue(ctx, txContextKey{}, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// Executor returns the transaction in context or the fallback pool executor.
func Executor(ctx context.Context, fallback DBTX) DBTX {
	if tx, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return fallback
}
