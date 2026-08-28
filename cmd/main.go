package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	accountrepository "github.com/thec1oud/billing/internal/account/repository"
	accountservice "github.com/thec1oud/billing/internal/account/service"
	"github.com/thec1oud/billing/internal/api"
	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/database"
	"github.com/thec1oud/billing/internal/infra"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/plan"
	eventrepository "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
	subscriptionrepository "github.com/thec1oud/billing/internal/subscription/repository"
	subscriptionservice "github.com/thec1oud/billing/internal/subscription/service"
	"github.com/thec1oud/billing/internal/tariff"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logClosers, err := logger.InitGlobalLogger(cfg)
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer func() {
		for _, c := range logClosers {
			_ = c.Close()
		}
	}()

	deps, err := infra.InitDependencies(ctx, cfg)
	if err != nil {
		return fmt.Errorf("infrastructure: %w", err)
	}
	defer func() {
		if deps.Rabbit != nil {
			_ = deps.Rabbit.Close()
		}
		if deps.Redis != nil {
			_ = deps.Redis.Close()
		}
		if deps.Pool != nil {
			deps.Pool.Close()
		}
		if deps.DB != nil {
			_ = deps.DB.Close(ctx)
		}
	}()
	if err := database.RunMigrations(deps.DB); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	port, err := strconv.Atoi(cfg.AppPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid APP_PORT %q", cfg.AppPort)
	}
	accountRepo := accountrepository.New(deps.Pool)
	events := eventservice.NewService(eventrepository.NewPostgresEventStore(deps.Pool))
	router := api.NewRouter(api.Services{
		Accounts: accountservice.NewWithEvents(accountRepo, events),
		Subscriptions: subscriptionservice.New(
			subscriptionrepository.New(deps.Pool),
			accountRepo,
			plan.NewPostgresRepository(deps.Pool),
			tariff.NewPostgresRepository(deps.Pool),
		),
	})
	server := api.NewServer(port, router)
	errCh := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	slog.Info("billing HTTP server started", "port", port)

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	slog.Info("billing service shut down")
	return nil
}
