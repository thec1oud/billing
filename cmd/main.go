package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/database"
	"github.com/thec1oud/billing/internal/infra"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/messaging"
	attemptRepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptSvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	"github.com/thec1oud/billing/internal/ppi/handler"
	"github.com/thec1oud/billing/internal/ppi/repository"
	"github.com/thec1oud/billing/internal/ppi/service"
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

	// 4. Deferred resource teardown (LIFO order: Rabbit -> Redis -> DB)
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
		if deps.DB != nil {
			_ = deps.DB.Close(ctx)
		}
	}()

	// 5. Run database migrations
	if err := database.RunMigrations(deps.DB); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	// 6. Initialize Messaging Broker Topology
	rabbitBroker, err := messaging.NewRabbitBroker(deps.Rabbit)
	if err != nil {
		return fmt.Errorf("messaging broker: %w", err)
	}
	if err := rabbitBroker.InitTopology(ctx); err != nil {
		return fmt.Errorf("broker topology init: %w", err)
	}
	defer rabbitBroker.Close()

	// 7. Initialize Payment Attempt Repository & Service
	paymentAttemptRepo := attemptRepo.NewPostgresRepository()
	paymentAttemptSvc := attemptSvc.NewService(paymentAttemptRepo)

	// 8. Initialize PPI Webhook Repository, PPIService, and WebhookHandler
	ppiRepo := repository.NewPostgresRepository()
	ppiService := service.NewService(deps.DB, ppiRepo, paymentAttemptSvc)
	ppiService.RegisterAdapter(fake.NewFakeAdapter())

	webhookHandler := handler.NewWebhookHandler(ppiService, rabbitBroker)

	// 9. Configure HTTP Router & Mount Webhook Listener
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/webhooks/{provider}", webhookHandler)

	// Set up component logger for the main process
	log := logger.ForComponent("main")

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.AppPort),
		Handler: logger.RequestLogger(mux),
	}

	// 10. Start HTTP Server Listener in background goroutine
	go func() {
		log.Info("HTTP Webhook server listening", slog.String("addr", srv.Addr), slog.String("endpoint", "POST /api/v1/webhooks/{provider}"))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP Server crashed", logger.Err(err))
		}
	}()

	log.Info("All background services wired successfully. Application layer online.", slog.Int("port", cfg.AppPort))

	// 11. Block process until SIGINT/SIGTERM for graceful shutdown
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-sigCtx.Done()
	log.Info("Shutting down billing service gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP Server Shutdown error", logger.Err(err))
	}

	log.Info("Billing service stopped successfully.")
	return nil
}
