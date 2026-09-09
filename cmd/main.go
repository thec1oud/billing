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
	accountrepo "github.com/thec1oud/billing/internal/account/repository"
	accountsvc "github.com/thec1oud/billing/internal/account/service"
	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/database"
	"github.com/thec1oud/billing/internal/infra"
	"github.com/thec1oud/billing/internal/infra/api"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/messaging"
	invoiceRepo "github.com/thec1oud/billing/internal/invoice/repository"
	invoicesvc "github.com/thec1oud/billing/internal/invoice/service"
	"github.com/thec1oud/billing/internal/invoice/statemachine"
	attemptRepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptSvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	planmodel "github.com/thec1oud/billing/internal/plan/model"
	planrepo "github.com/thec1oud/billing/internal/plan/repository"
	planservice "github.com/thec1oud/billing/internal/plan/service"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	"github.com/thec1oud/billing/internal/ppi/repository"
	"github.com/thec1oud/billing/internal/ppi/service"
	itemmodel "github.com/thec1oud/billing/internal/purchasable_item/model"
	itemservice "github.com/thec1oud/billing/internal/purchasable_item/service"
	eventrepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	eventsvc "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	"github.com/thec1oud/billing/internal/shared/statemachine/outbox"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	smRepo "github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/statemachine/scheduler"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"
	subscriptionrepo "github.com/thec1oud/billing/internal/subscription/repository"
	subscriptionsvc "github.com/thec1oud/billing/internal/subscription/service"
	subscriptionstatemachine "github.com/thec1oud/billing/internal/subscription/statemachine"
	tariffmodel "github.com/thec1oud/billing/internal/tariff/model"
	tariffrepo "github.com/thec1oud/billing/internal/tariff/repository"
	tariffservice "github.com/thec1oud/billing/internal/tariff/service"
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

	planRepo := planrepo.NewPostgresRepository(deps.Pool)
	planSvc := planservice.NewService(planRepo)

	tariffRepo := tariffrepo.NewPostgresRepository(deps.Pool)

	subscriptionRepo := subscriptionrepo.New(deps.Pool)
	subscriptionstatemachine.RegisterStateMachineActions(smRegistry, subscriptionRepo)
	_, err = loader.Publish(ctx, deps.Pool, smRepository, smRegistry, subscriptionstatemachine.BuildSubscriptionDefinitionSpec())
	if err != nil && !strings.Contains(err.Error(), "23505") {
		return fmt.Errorf("failed to bootstrap subscription state machine: %w", err)
	}
	subscriptionSvc := subscriptionsvc.New(subscriptionRepo, accountRepo, planRepo, tariffRepo, smEngine)

	eventRepo := eventrepo.NewPostgresEventStore(deps.Pool)
	eventSvc := eventsvc.NewService(eventRepo)
	invoiceSvc := invoicesvc.NewService(deps.Pool, eventSvc, ppiService, invoiceRepository, smEngine)

	// 11. Configure & Start HTTP Server
	srv := api.NewServer(cfg, api.Deps{
		Pool:                deps.Pool,
		AccountService:      accountSvc,
		PlanService:         planSvc,
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

func SeedCatalog(
	ctx context.Context,
	pool *pgxpool.Pool,
	planSvc *planservice.Service,
	tariffSvc *tariffservice.Service,
	itemSvc *itemservice.Service,
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
		tariffmodel.TariffTypeFlatFee,
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

	p, err := planSvc.CreatePlan(ctx, tx, planmodel.Plan{
		PlanCode:              "USAGE_PLAN_A",
		LegacyPricePolicyCode: planmodel.LegacyPolicyKeepForever,
	})
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}

	_, err = planSvc.CreatePlanDuration(ctx, tx, planmodel.PlanDuration{
		PlanID:   p.ID,
		TariffID: t.ID,
		Duration: 30 * 24 * time.Hour,
	})
	if err != nil {
		return fmt.Errorf("create plan duration: %w", err)
	}

	_, err = itemSvc.Create(ctx, tx, itemmodel.PurchasableItem{
		ItemCode:     "API_REQUEST",
		ItemTypeCode: itemmodel.ItemTypePlan,
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
