package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/thec1oud/billing/internal/config"
)

// loadEnv searches upward from the current working directory until it finds .env
func loadEnv() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	dir := wd
	for {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			// Overload forces values from .env over empty environment variables
			if err := godotenv.Overload(envPath); err != nil {
				return fmt.Errorf("overload env from %s: %w", envPath, err)
			}
			return nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return fmt.Errorf(".env file not found searching up from %s", wd)
}

func GetTestPool() (*pgxpool.Pool, error) {
	if err := loadEnv(); err != nil {
		return nil, err
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	dbHost := cfg.DBHost
	if dbHost == "app-db" || dbHost == "db" {
		dbHost = "localhost"
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser,
		cfg.DBPassword,
		dbHost,
		cfg.DBPort,
		cfg.DBName,
	)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test db: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping test db: %w", err)
	}

	return pool, nil
}
