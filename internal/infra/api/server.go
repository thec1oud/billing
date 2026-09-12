package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	accountHandlers "github.com/thec1oud/billing/internal/account/handler"
	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra/logger"
	invoiceHandlers "github.com/thec1oud/billing/internal/invoice/handler"
	planHandlers "github.com/thec1oud/billing/internal/plan/handler"
	ppiHandlers "github.com/thec1oud/billing/internal/ppi/handler"
	itemHandlers "github.com/thec1oud/billing/internal/purchasable_item/handler"
	subscriptionHandlers "github.com/thec1oud/billing/internal/subscription/handler"
	tariffHandlers "github.com/thec1oud/billing/internal/tariff/handler"
)

type Server struct {
	srv *http.Server
	log *slog.Logger
}

// NewServer wires all module route handlers onto the mux and configures the HTTP server.
func NewServer(cfg *config.Config, deps Deps) *Server {
	mux := http.NewServeMux()

	if deps.AccountService != nil {
		accountHandler := accountHandlers.NewAccountHandler(deps.AccountService)
		mux.HandleFunc("POST /api/v1/accounts", accountHandler.HandleCreate)
		mux.HandleFunc("GET /api/v1/accounts/{accountID}", accountHandler.HandleGet)
		mux.HandleFunc("POST /api/v1/accounts/{accountID}/activate", accountHandler.HandleActivate)
		mux.HandleFunc("POST /api/v1/accounts/{accountID}/suspend", accountHandler.HandleSuspend)
		mux.HandleFunc("POST /api/v1/accounts/{accountID}/reactivate", accountHandler.HandleReactivate)
		mux.HandleFunc("POST /api/v1/accounts/{accountID}/close", accountHandler.HandleClose)
		mux.HandleFunc("POST /api/v1/accounts/{accountID}/payment-methods", accountHandler.HandleAddPaymentMethod)
	}

	if deps.TariffService != nil && deps.Pool != nil {
		tariffHandler := tariffHandlers.NewTariffHandler(deps.TariffService)
		mux.HandleFunc("POST /api/v1/tariffs", tariffHandler.HandleCreateTariff)
	}

	if deps.PlanService != nil && deps.Pool != nil {
		planHandler := planHandlers.NewPlanHandler(deps.Pool, deps.PlanService, deps.PurchasableItemService)
		mux.HandleFunc("POST /api/v1/plans", planHandler.HandleCreatePlan)
		mux.HandleFunc("GET /api/v1/plans", planHandler.HandleListPlans)
	}

	if deps.PurchasableItemService != nil {
		itemHandler := itemHandlers.NewPurchasableItemHandler(deps.PurchasableItemService)
		mux.HandleFunc("GET /api/v1/purchasable-items", itemHandler.HandleList)
	}

	if deps.InvoiceService != nil {
		invoiceHandler := invoiceHandlers.NewInvoiceHandler(deps.InvoiceService, deps.PPIService, deps.PurchasableItemService)
		if deps.PurchasableItemService != nil {
			mux.HandleFunc("POST /api/v1/dev/invoices/generate", invoiceHandler.HandleGenerateDevInvoice)
		}
		if deps.PPIService != nil {
			mux.HandleFunc("POST /api/v1/invoices/{invoiceID}/pay", invoiceHandler.HandlePayInvoice)
		}
		mux.HandleFunc("GET /api/v1/invoices/{invoiceID}", invoiceHandler.HandleGetInvoice)
		mux.HandleFunc("GET /api/v1/accounts/{id}/invoices", invoiceHandler.HandleListInvoices)
	}

	// Subscription routes
	if deps.SubscriptionService != nil {
		subscriptionHandler := subscriptionHandlers.NewSubscriptionHandler(deps.SubscriptionService)
		mux.HandleFunc("POST /api/v1/subscriptions", subscriptionHandler.HandleCreateSubscription)
		mux.HandleFunc("GET /api/v1/subscriptions/{subscriptionID}", subscriptionHandler.HandleGetSubscription)
		mux.HandleFunc("GET /api/v1/accounts/{id}/subscriptions", subscriptionHandler.HandleListAccountSubscriptions)
	}

	// PPI routes
	if deps.PPIService != nil {
		ppiHandler := ppiHandlers.NewPPIHandler(deps.PPIService, deps.Broker)
		mux.HandleFunc("POST /api/v1/payments/charge", ppiHandler.HandleAttemptPayment)
		mux.HandleFunc("POST /api/v1/webhooks/{provider}", ppiHandler.HandleWebhook)
	}

	var handler http.Handler = mux
	if deps.Pool != nil {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "db_pool", deps.Pool)
			mux.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	return &Server{
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%s", cfg.AppPort),
			Handler: logger.RequestLogger(handler),
		},
		log: logger.ForComponent("http_server"),
	}
}

// Start launches the HTTP server listener in a non-blocking background goroutine.
func (s *Server) Start() {
	go func() {
		s.log.Info("HTTP server listening", slog.String("addr", s.srv.Addr))
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("HTTP server crashed", logger.Err(err))
		}
	}()
}

// Shutdown gracefully stops the server, waiting for outstanding connection processing.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down HTTP server gracefully...")
	return s.srv.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	return s.srv.Handler
}
