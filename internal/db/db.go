// Package db provides the shared Postgres connection pool and migration
// runner used by both the API and worker services.
package db

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx driver for migrate
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Config holds Postgres connection settings, normally sourced from env vars.
type Config struct {
	DSN string // e.g. postgres://user:pass@host:5432/dbname?sslmode=disable
}

// New creates a pgx connection pool and verifies connectivity with a ping.
func New(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	poolCfg.MaxConns = 10
	poolCfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}

// RunMigrations applies all pending "up" migrations embedded in this package.
// It is safe to call from both services on startup; golang-migrate no-ops
// if the schema is already current.
//
// golang-migrate's pgx-v5 driver registers itself under the "pgx5://" URL
// scheme, not "postgres://" (which pgxpool itself accepts directly). So we
// rewrite just the scheme here for the migrator; the DSN passed around
// everywhere else in the app stays a normal postgres:// / postgresql:// URL.
func RunMigrations(dsn string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load migration source: %w", err)
	}

	migrateDSN := dsn
	switch {
	case strings.HasPrefix(dsn, "postgres://"):
		migrateDSN = "pgx5://" + strings.TrimPrefix(dsn, "postgres://")
	case strings.HasPrefix(dsn, "postgresql://"):
		migrateDSN = "pgx5://" + strings.TrimPrefix(dsn, "postgresql://")
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, migrateDSN)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
