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
)

type Server struct {
	srv *http.Server
	log *slog.Logger
}

// NewServer wires all module route handlers onto the mux and configures the HTTP
// server. To add a new module's endpoints: import its handler package here,
// add its service to Deps, then register its routes below.
func NewServer(cfg *config.Config, deps Deps) *Server {
	mux := http.NewServeMux()

	// Account routes
	if deps.AccountService != nil {
		accountHandler := accountHandlers.NewAccountHandler(deps.AccountService)
		mux.HandleFunc("POST /api/v1/accounts", accountHandler.HandleCreate)
		mux.HandleFunc("POST /api/v1/accounts/{id}/activate", accountHandler.HandleActivate)
	}

	// Catalog routes
	if deps.PlanService != nil {
		planHandler := planHandlers.NewPlanHandler(deps.PlanService)
		mux.HandleFunc("GET /api/v1/plans", planHandler.HandleList)
	}
	if deps.PurchasableItemService != nil {
		itemHandler := itemHandlers.NewPurchasableItemHandler(deps.PurchasableItemService)
		mux.HandleFunc("GET /api/v1/purchasable-items", itemHandler.HandleList)
	}

	// Subscription routes
	if deps.SubscriptionService != nil {
		subscriptionHandler := subscriptionHandlers.NewSubscriptionHandler(deps.SubscriptionService)
		mux.HandleFunc("POST /api/v1/subscriptions", subscriptionHandler.HandleCreate)
	}

	// Invoice routes
	if deps.InvoiceService != nil {
		invoiceHandler := invoiceHandlers.NewInvoiceHandler(deps.InvoiceService)
		mux.HandleFunc("POST /api/v1/invoices", invoiceHandler.HandleCreateDraft)
		mux.HandleFunc("POST /api/v1/invoices/{id}/finalize", invoiceHandler.HandleFinalize)
		mux.HandleFunc("GET /api/v1/invoices/{id}", invoiceHandler.HandleGet)
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
