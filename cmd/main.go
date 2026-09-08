package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/database"
	"github.com/thec1oud/billing/internal/infra"
	"github.com/thec1oud/billing/internal/infra/api"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/messaging"
	invoiceRepo "github.com/thec1oud/billing/internal/invoice/repository"
	"github.com/thec1oud/billing/internal/invoice/statemachine"
	attemptRepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptSvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	"github.com/thec1oud/billing/internal/ppi/repository"
	"github.com/thec1oud/billing/internal/ppi/service"
	"github.com/thec1oud/billing/internal/seed/dev"
	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	"github.com/thec1oud/billing/internal/shared/statemachine/outbox"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	smRepo "github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/statemachine/scheduler"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"

	accountrepo "github.com/thec1oud/billing/internal/account/repository"
	accountsvc "github.com/thec1oud/billing/internal/account/service"
	invoicesvc "github.com/thec1oud/billing/internal/invoice/service"
	"github.com/thec1oud/billing/internal/plan"
	purchasableitem "github.com/thec1oud/billing/internal/purchasable_item"
	eventrepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	eventsvc "github.com/thec1oud/billing/internal/shared/eventstore/service"
	subscriptionrepo "github.com/thec1oud/billing/internal/subscription/repository"
	subscriptionsvc "github.com/thec1oud/billing/internal/subscription/service"
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

	// 4. Run database migrations
	if err := database.RunMigrations(deps.DB); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	// 6. Block process until SIGINT/SIGTERM for background contexts
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 7. Wire the state machine engine's background workers.
	smRegistry := registry.New()
	smRepository := smRepo.NewPostgresRepository(deps.Pool)
	smEngine := engine.NewEngine(deps.Pool, smRepository, smRegistry, engine.WithScripting(scripting.NewPool(0, 0)))

	// Register invoice state machine actions & publish definition spec
	invoiceRepository := invoiceRepo.NewPostgresRepository(deps.Pool)
	statemachine.RegisterStateMachineActions(smRegistry, invoiceRepository)

	_, err = loader.Publish(ctx, deps.Pool, smRepository, smRegistry, statemachine.BuildInvoiceDefinitionSpec())
	if err != nil && !strings.Contains(err.Error(), "23505") {
		return fmt.Errorf("failed to bootstrap invoice state machine: %w", err)
	}

	smPublisher := outbox.NewPublisher(deps.Pool, smRepository, smRegistry, outbox.Config{})
	smPublisher.Start(sigCtx)
	defer smPublisher.Stop()

	smScheduler := scheduler.NewPoller(smEngine, smRepository, scheduler.Config{})
	smScheduler.Start(sigCtx)
	defer smScheduler.Stop()

	// 8. Initialize Messaging Broker Topology
	rabbitBroker, err := messaging.NewRabbitBroker(deps.Rabbit)
	if err != nil {
		return fmt.Errorf("messaging broker: %w", err)
	}
	if err := rabbitBroker.InitTopology(ctx); err != nil {
		return fmt.Errorf("broker topology init: %w", err)
	}
	defer rabbitBroker.Close()

	// 9. Initialize Payment Attempt Repository & Service
	paymentAttemptRepo := attemptRepo.NewPostgresRepository(deps.Pool)
	paymentAttemptSvc := attemptSvc.NewService(paymentAttemptRepo)

	// 10. Initialize PPI Webhook Repository & Service
	ppiRepo := repository.NewPostgresRepository()
	ppiService := service.NewService(deps.DB, ppiRepo, paymentAttemptSvc)
	ppiService.RegisterAdapter(fake.NewFakeAdapter())

	// Initialize other domain services
	accountRepo := accountrepo.New(deps.Pool)
	accountSvc := accountsvc.New(accountRepo)

	planRepo := plan.NewPostgresRepository(deps.Pool)
	itemRepo := purchasableitem.NewPostgresRepository(deps.Pool)
	itemSvc := purchasableitem.NewService(itemRepo)
	planSvc := plan.NewService(deps.Pool, planRepo, itemSvc)

	tariffRepo := tariff.NewPostgresRepository(deps.Pool)
	tariffSvc := tariff.NewService(tariffRepo)

	// 5. Seed essential development data
	if cfg.AppEnv != "production" {
		if err := dev.Seed(ctx, deps.Pool, planSvc, tariffSvc, logger.ForComponent("dev_seeder")); err != nil {
			return fmt.Errorf("failed to run dev seeder: %w", err)
		}
	}

	subscriptionRepo := subscriptionrepo.New(deps.Pool)
	subscriptionSvc := subscriptionsvc.New(subscriptionRepo, accountRepo, planRepo, tariffRepo)

	eventRepo := eventrepo.NewPostgresEventStore(deps.Pool)
	eventSvc := eventsvc.NewService(eventRepo)
	invoiceSvc := invoicesvc.NewService(deps.Pool, eventSvc, invoiceRepository, smEngine)

	// 11. Configure & Start HTTP Server
	srv := api.NewServer(cfg, api.Deps{
		Pool:                deps.Pool,
		AccountService:      accountSvc,
		PlanService:         planSvc,
		TariffService:       tariffSvc,
		SubscriptionService: subscriptionSvc,
		InvoiceService:      invoiceSvc,
		PPIService:          ppiService,
		Broker:              rabbitBroker,
	})
	srv.Start()

	// Set up component logger for the main process
	log := logger.ForComponent("main")
	log.Info("All background services wired successfully. Application layer online.", slog.String("port", cfg.AppPort))

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
