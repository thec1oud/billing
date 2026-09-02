package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	accountHandler "github.com/thec1oud/billing/internal/account/handler"
	"github.com/thec1oud/billing/internal/config"
	invoiceHandler "github.com/thec1oud/billing/internal/invoice/handler"
	"github.com/thec1oud/billing/internal/infra/logger"
	planHandler "github.com/thec1oud/billing/internal/plan/handler"
	ppiHandlers "github.com/thec1oud/billing/internal/ppi/handler"
	subHandler "github.com/thec1oud/billing/internal/subscription/handler"
	tariffHandler "github.com/thec1oud/billing/internal/tariff/handler"
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
		accountAPI := accountHandler.NewAccountHandler(deps.AccountService)
		mux.HandleFunc("POST /api/v1/accounts", accountAPI.HandleCreateAccount)
		mux.HandleFunc("POST /api/v1/accounts/{id}/activate", accountAPI.HandleActivateAccount)
	}

	// Tariff routes
	if deps.TariffService != nil {
		tariffAPI := tariffHandler.NewTariffHandler(deps.TariffService)
		mux.HandleFunc("POST /api/v1/tariffs", tariffAPI.HandleCreateTariff)
	}

	// Plan routes
	if deps.PlanService != nil {
		planAPI := planHandler.NewPlanHandler(deps.PlanService)
		mux.HandleFunc("POST /api/v1/plans", planAPI.HandleCreatePlan)
	}

	// Subscription routes
	if deps.SubscriptionService != nil {
		subAPI := subHandler.NewSubscriptionHandler(deps.SubscriptionService)
		mux.HandleFunc("POST /api/v1/subscriptions", subAPI.HandleCreateSubscription)
	}

	// Invoice routes
	if deps.InvoiceService != nil {
		invHandler := invoiceHandler.NewInvoiceHandler(deps.InvoiceService, deps.PPIService)
		mux.HandleFunc("GET /api/v1/invoices/{id}", invHandler.HandleGetInvoice)
		mux.HandleFunc("GET /api/v1/accounts/{id}/invoices", invHandler.HandleListInvoices)
		mux.HandleFunc("POST /api/v1/invoices/{id}/pay", invHandler.HandlePayInvoice)
	}

	// PPI routes
	ppiWebhook := ppiHandlers.NewWebhookHandler(deps.PPIService, deps.Broker)
	mux.HandleFunc("POST /api/v1/webhooks/{provider}", ppiWebhook.HandleWebhook)

	return &Server{
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%s", cfg.AppPort),
			Handler: logger.RequestLogger(mux),
		},
		log: logger.ForComponent("http_server"),
	}
}

// Handler returns the underlying http.Handler.
func (s *Server) Handler() http.Handler {
	return s.srv.Handler
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
