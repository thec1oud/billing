package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/database"
	"github.com/thec1oud/billing/internal/infra"
	"github.com/thec1oud/billing/internal/infra/logger"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	deps := infra.InitDependencies(ctx, cfg)
	logClosers, err := logger.InitGlobalLogger(cfg)
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer func() {
		for _, c := range logClosers {
			_ = c.Close()
		}
	}()
	defer func() { _ = deps.DB.Close(ctx) }()
	defer func() { _ = deps.Redis.Close() }()
	defer func() { _ = deps.Rabbit.Close() }()

	if err := database.RunMigrations(deps.DB); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	slog.Info("All background services wired. Starting application layer...", "port", cfg.AppPort)
	return nil
}
