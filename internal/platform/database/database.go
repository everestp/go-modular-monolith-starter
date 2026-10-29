// Package database provides PostgreSQL connection pooling helpers.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config wraps the pgx pool configuration.
type Config struct {
	URL          string
	MinConns     int32
	MaxConns     int32
	MaxLifetime  time.Duration
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
	poolCfg.HealthCheckPeriod = 30 * time.Second
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
