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

	"github.com/jackc/pgx/v5/pgxpool"
	accountRepo "github.com/thec1oud/billing/internal/account/repository"
	accountSvc "github.com/thec1oud/billing/internal/account/service"
	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/database"
	"github.com/thec1oud/billing/internal/infra"
	"github.com/thec1oud/billing/internal/infra/api"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/messaging"
	invoiceRepo "github.com/thec1oud/billing/internal/invoice/repository"
	invoiceSvc "github.com/thec1oud/billing/internal/invoice/service"
	"github.com/thec1oud/billing/internal/invoice/statemachine"
	attemptRepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptSvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/plan"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	"github.com/thec1oud/billing/internal/ppi/repository"
	"github.com/thec1oud/billing/internal/ppi/service"
	"github.com/thec1oud/billing/internal/purchasable_item"
	eventRepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	eventSvc "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	"github.com/thec1oud/billing/internal/shared/statemachine/outbox"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	smRepo "github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/statemachine/scheduler"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"
	subRepo "github.com/thec1oud/billing/internal/subscription/repository"
	subSvc "github.com/thec1oud/billing/internal/subscription/service"
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

	// 10.5 Initialize Core Domain Services
	accountRepository := accountRepo.New(deps.Pool)
	accountService := accountSvc.New(accountRepository)

	planRepository := plan.NewPostgresRepository(deps.Pool)
	planService := plan.NewService(planRepository)

	tariffRepository := tariff.NewPostgresRepository(deps.Pool)
	tariffService := tariff.NewService(tariffRepository)

	purchasableItemRepository := purchasable_item.NewPostgresRepository(deps.Pool)
	purchasableItemService := purchasable_item.NewService(purchasableItemRepository)

	subscriptionRepository := subRepo.New(deps.Pool)
	subscriptionService := subSvc.New(subscriptionRepository, accountRepository, planRepository, tariffRepository)

	eventRepository := eventRepo.NewPostgresEventStore(deps.Pool)
	eventService := eventSvc.NewService(eventRepository)

	invoiceService := invoiceSvc.NewService(deps.Pool, eventService, ppiService, invoiceRepository, smEngine)

	if os.Getenv("SEED_DB") == "true" {
		if err := SeedCatalog(ctx, deps.Pool, planService, tariffService, purchasableItemService); err != nil {
			return fmt.Errorf("failed to seed catalog: %w", err)
		}
	}

	// 11. Configure & Start HTTP Server
	srv := api.NewServer(cfg, api.Deps{
		PPIService:             ppiService,
		Broker:                 rabbitBroker,
		AccountService:         accountService,
		PlanService:            planService,
		PurchasableItemService: purchasableItemService,
		SubscriptionService:    subscriptionService,
		InvoiceService:         invoiceService,
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

func SeedCatalog(
	ctx context.Context,
	pool *pgxpool.Pool,
	planSvc *plan.Service,
	tariffSvc *tariff.Service,
	itemSvc *purchasable_item.Service,
) error {
	log := logger.ForComponent("seeder")

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	amt, _ := money.New(1000, "ETB") // 10.00 ETB per unit

	t, err := tariffSvc.CreateTariff(
		ctx,
		tx,
		"STANDARD_USAGE_V1",
		"Standard Usage Pricing",
		"Standard per-unit pricing",
		tariff.TariffTypeFlatFee,
		amt,
		nil,
		nil,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			log.Info("Catalog already seeded")
			return nil
		}
		return fmt.Errorf("create tariff: %w", err)
	}

	p, err := planSvc.CreatePlan(ctx, tx, plan.Plan{
		PlanCode:              "USAGE_PLAN_A",
		LegacyPricePolicyCode: plan.LegacyPolicyKeepForever,
	})
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}

	_, err = planSvc.CreatePlanDuration(ctx, tx, plan.PlanDuration{
		PlanID:   p.ID,
		TariffID: t.ID,
		Duration: 30 * 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("create plan duration: %w", err)
	}

	_, err = itemSvc.Create(ctx, tx, purchasable_item.PurchasableItem{
		ItemCode:     "API_REQUEST",
		ItemTypeCode: purchasable_item.ItemTypePlan,
		Name:         "API Requests",
		PlanID:       &p.ID,
		IsActive:     true,
	})
	if err != nil {
		return fmt.Errorf("create item: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed tx: %w", err)
	}

	log.Info("Successfully seeded DB with reference catalog (tariffs, plans, items).")
	return nil
}
