package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/thec1oud/billing/internal/config"
)

func NewPostgresConnection(ctx context.Context, cfg *config.Config) (*pgx.Conn, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return conn, nil
}
