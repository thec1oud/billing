package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/config"
)

func dsn(cfg *config.Config) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
}

func NewPostgresConnection(ctx context.Context, cfg *config.Config) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, dsn(cfg))
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return conn, nil
}

// NewPostgresPool opens a pgxpool.Pool for repositories that compose multi-query
// transactions (e.g. the state machine engine), as opposed to the single
// long-lived *pgx.Conn used for migrations.
func NewPostgresPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn(cfg))
	if err != nil {
		return nil, fmt.Errorf("database pool connection failed: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database pool ping failed: %w", err)
	}

	return pool, nil
}
