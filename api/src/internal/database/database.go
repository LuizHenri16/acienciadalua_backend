// Package database owns the PostgreSQL connection pool and schema migrations.
package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/acienciadalua/api/migrations"
	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// SanitizeDatabaseURL removes query parameters that pgx does not understand
// (notably `?schema=public`, which PostgreSQL rejects as an unknown startup
// parameter).
func SanitizeDatabaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	query := parsed.Query()
	for _, key := range []string{
		"schema",
		"connection_limit",
		"pool_timeout",
		"pgbouncer",
		"sslidentity",
		"sslaccept",
	} {
		query.Del(key)
	}
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

// NewPool creates and verifies the pgx connection pool.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	dsn, err := SanitizeDatabaseURL(databaseURL)
	if err != nil {
		return nil, err
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing DATABASE_URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connecting to PostgreSQL: %w", err)
	}

	return pool, nil
}

// RunMigrations applies the embedded golang-migrate migrations.
//
// The baseline migration is idempotent, so it can also be applied to a database
// that already has the schema. golang-migrate records its own version in
// `schema_migrations`.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	dsn, err := SanitizeDatabaseURL(databaseURL)
	if err != nil {
		return err
	}

	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parsing DATABASE_URL for migrations: %w", err)
	}

	sqlDB := stdlib.OpenDB(*connConfig)
	defer sqlDB.Close()

	driver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("loading embedded migrations: %w", err)
	}

	runner, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("creating migration runner: %w", err)
	}
	defer func() {
		if sourceErr, dbErr := runner.Close(); sourceErr != nil || dbErr != nil {
			logger.Warn("error closing migration runner", "source_error", sourceErr, "database_error", dbErr)
		}
	}()

	if err := runner.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("database migrations already up to date")
			return nil
		}
		return fmt.Errorf("applying migrations: %w", err)
	}

	version, dirty, err := runner.Version()
	if err != nil {
		logger.Info("database migrations applied")
		return nil
	}
	logger.Info("database migrations applied", "version", version, "dirty", dirty)

	return nil
}
