package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra"
	"github.com/thec1oud/billing/internal/infra/logger"
)

func main() {
	ctx := context.Background()

	// Load config vars from the env file
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to initialize config variables", "error", err)
		os.Exit(1)
	}

	deps := infra.InitDependencies(ctx, cfg)
	logClosers, err := logger.InitGlobalLogger(cfg)
	if err != nil {
		log.Fatalf("Logger boot failed: %v", err)
	}

	// Gracefully flush/close all underlying adapters upon application shutdown
	defer func() {
		for _, closer := range logClosers {
			_ = closer.Close()
		}
	}()

	//clean up when the main loop exits
	defer func() { _ = deps.DB.Close(ctx) }()
	defer func() { _ = deps.Redis.Close() }()
	defer func() { _ = deps.Rabbit.Close() }()

	slog.Info("All background services wired. Starting application layer...", "port", cfg.AppPort)

}
