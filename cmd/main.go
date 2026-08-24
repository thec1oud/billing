package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/database"
	"github.com/thec1oud/billing/internal/infra"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	"github.com/thec1oud/billing/internal/shared/statemachine/outbox"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/statemachine/scheduler"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"
)

func main() {
	fmt.Println("----> STARTING BILLING SERVICE <----")
	if err := run(); err != nil {

		fmt.Fprintf(os.Stderr, "FATAL ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// 2. Initialize global logger FIRST so all downstream logs use the configured pipeline
	logClosers, err := logger.InitGlobalLogger(cfg)
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer func() {
		for _, c := range logClosers {
			_ = c.Close()
		}
	}()

	// 3. Initialize infrastructural dependencies
	deps, err := infra.InitDependencies(ctx, cfg)
	if err != nil {
		return fmt.Errorf("infrastructure: %w", err)
	}

	// 4. Deferred resource teardown (LIFO order: Rabbit -> Redis -> Pool -> DB)
	defer func() {
		if deps.Rabbit != nil {
			_ = deps.Rabbit.Close()
		}
	}()
	defer func() {
		if deps.Redis != nil {
			_ = deps.Redis.Close()
		}
	}()
	defer func() {
		if deps.Pool != nil {
			deps.Pool.Close()
		}
	}()
	defer func() {
		if deps.DB != nil {
			_ = deps.DB.Close(ctx)
		}
	}()

	// 5. Run database migrations
	if err := database.RunMigrations(deps.DB); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	// 6. Block process until SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 7. Wire the state machine engine's background workers. The guard/action
	// registry is empty here — no domain (invoice, payment_attempt, subscription)
	// is wired onto the state machine engine yet; that's follow-up work. A
	// future domain package registers its guards/actions into this registry at
	// startup before any definition referencing them is published.
	smRegistry := registry.New()
	smRepo := repository.NewPostgresRepository(deps.Pool)
	smEngine := engine.NewEngine(deps.Pool, smRepo, smRegistry, engine.WithScripting(scripting.NewPool(0, 0)))

	smPublisher := outbox.NewPublisher(deps.Pool, smRepo, smRegistry, outbox.Config{})
	smPublisher.Start(ctx)
	defer smPublisher.Stop()

	smScheduler := scheduler.NewPoller(smEngine, smRepo, scheduler.Config{})
	smScheduler.Start(ctx)
	defer smScheduler.Stop()

	slog.Info("All background services wired. Starting application layer...", "port", cfg.AppPort)

	// START YOUR SERVER / CONSUMER HERE (e.g. go httpApp.Start(...) or go worker.Listen(ctx))

	<-ctx.Done()
	slog.Info("Shutting down billing service...")

	return nil
}
