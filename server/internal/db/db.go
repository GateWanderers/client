package db

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/migrations"
)

// Connect creates a new pgxpool connection pool and returns it.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pool.Ping: %w", err)
	}
	return pool, nil
}

// RunMigrations runs all pending up migrations using the embedded SQL files.
func RunMigrations(databaseURL string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("iofs.New: %w", err)
	}

	// golang-migrate's pgx/v5 driver expects the URL scheme "pgx5://"
	pgx5URL := "pgx5://" + databaseURL[len("postgres://"):]

	m, err := migrate.NewWithSourceInstance("iofs", src, pgx5URL)
	if err != nil {
		return fmt.Errorf("migrate.NewWithSourceInstance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate.Up: %w", err)
	}
	return nil
}
