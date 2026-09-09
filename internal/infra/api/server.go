package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	accountmodel "github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	planmodel "github.com/thec1oud/billing/internal/plan/model"
	ppiHandlers "github.com/thec1oud/billing/internal/ppi/handler"
	itemHandlers "github.com/thec1oud/billing/internal/purchasable_item/handler"
	itemmodel "github.com/thec1oud/billing/internal/purchasable_item/model"
	"github.com/thec1oud/billing/internal/shared/money"
	subscriptionHandlers "github.com/thec1oud/billing/internal/subscription/handler"
	tariffmodel "github.com/thec1oud/billing/internal/tariff/model"
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

	if deps.AccountService != nil {
		mux.HandleFunc("POST /api/v1/accounts", handleCreateAccount(deps.AccountService))
		mux.HandleFunc("POST /api/v1/accounts/{accountID}/activate", handleActivateAccount(deps.AccountService))
	}

	if deps.TariffService != nil && deps.Pool != nil {
		mux.HandleFunc("POST /api/v1/tariffs", handleCreateTariff(deps))
	}

	if deps.PlanService != nil && deps.Pool != nil {
		mux.HandleFunc("POST /api/v1/plans", handleCreatePlan(deps))
	}

	if deps.PurchasableItemService != nil {
		itemHandler := itemHandlers.NewPurchasableItemHandler(deps.PurchasableItemService)
		mux.HandleFunc("GET /api/v1/purchasable-items", itemHandler.HandleList)
	}

	// Subscription routes
	if deps.SubscriptionService != nil {
		subscriptionHandler := subscriptionHandlers.NewSubscriptionHandler(deps.SubscriptionService)
		mux.HandleFunc("POST /api/v1/subscriptions", subscriptionHandler.HandleCreateSubscription)
	}

	// PPI routes
	if deps.PPIService != nil {
		ppiHandler := ppiHandlers.NewPPIHandler(deps.PPIService, deps.Broker)
		mux.HandleFunc("POST /api/v1/payments/charge", ppiHandler.HandleAttemptPayment)
		if deps.InvoiceService != nil {
			mux.HandleFunc("POST /api/v1/invoices/{invoiceID}/pay", handlePayInvoice(deps))
		}
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

func handleCreateAccount(s interface {
	Create(context.Context, accountmodel.CreateInput) (accountmodel.Account, error)
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input accountmodel.CreateInput
		if !decodeRequest(w, r, &input) {
			return
		}
		created, err := s.Create(r.Context(), input)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		response.Write(w, http.StatusCreated, created)
	}
}

func handleActivateAccount(s interface {
	Activate(context.Context, int64) (accountmodel.Account, error)
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parsePathID(w, r, "accountID")
		if !ok {
			return
		}
		account, err := s.Activate(r.Context(), id)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		response.Write(w, http.StatusOK, account)
	}
}

func handleCreateTariff(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Code        string                     `json:"code"`
			Name        string                     `json:"name"`
			Description string                     `json:"description"`
			TariffType  tariffmodel.TariffTypeCode `json:"tariff_type_code"`
			Amount      struct {
				AmountMinor int64  `json:"amount_minor"`
				Currency    string `json:"currency"`
			} `json:"amount"`
			Tiers    []tariffmodel.Tier `json:"tiers,omitempty"`
			Metadata []byte             `json:"metadata,omitempty"`
		}
		if !decodeRequest(w, r, &input) {
			return
		}

		tx, err := deps.Pool.Begin(r.Context())
		if err != nil {
			writeAPIError(w, err)
			return
		}
		defer tx.Rollback(r.Context())

		created, err := deps.TariffService.CreateTariff(
			r.Context(), tx, input.Code, input.Name, input.Description,
			input.TariffType, money.Money{AmountMinor: input.Amount.AmountMinor, Currency: money.Currency(input.Amount.Currency)},
			input.Tiers, input.Metadata,
		)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			writeAPIError(w, err)
			return
		}
		response.Write(w, http.StatusCreated, created)
	}
}

func handleCreatePlan(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input planmodel.Plan
		if !decodeRequest(w, r, &input) {
			return
		}

		tx, err := deps.Pool.Begin(r.Context())
		if err != nil {
			writeAPIError(w, err)
			return
		}
		defer tx.Rollback(r.Context())

		created, err := deps.PlanService.CreatePlan(r.Context(), tx, input)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		for i, duration := range input.Durations {
			duration.PlanID = created.ID
			createdDuration, durationErr := deps.PlanService.CreatePlanDuration(r.Context(), tx, duration)
			if durationErr != nil {
				writeAPIError(w, fmt.Errorf("create plan duration %d: %w", i, durationErr))
				return
			}
			created.Durations = append(created.Durations, createdDuration)
		}

		if deps.PurchasableItemService != nil {
			planID := created.ID
			_, err = deps.PurchasableItemService.Create(r.Context(), tx, itemmodel.PurchasableItem{
				ItemCode:     fmt.Sprintf("%s_v%d", created.PlanCode, created.Version),
				ItemTypeCode: itemmodel.ItemTypePlan,
				Name:         created.PlanCode,
				PlanID:       &planID,
				IsActive:     true,
			})
			if err != nil {
				writeAPIError(w, fmt.Errorf("create plan purchasable item: %w", err))
				return
			}
		}
		if err := tx.Commit(r.Context()); err != nil {
			writeAPIError(w, err)
			return
		}
		response.Write(w, http.StatusCreated, created)
	}
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{Code: "INVALID_REQUEST_BODY", Message: err.Error()})
		return false
	}
	return true
}

func parsePathID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	var id int64
	if _, err := fmt.Sscan(r.PathValue(key), &id); err != nil || id <= 0 {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{Code: "INVALID_ID", Message: key + " must be positive"})
		return 0, false
	}
	return id, true
}

func writeAPIError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must") {
		status = http.StatusBadRequest
	}
	response.Write(w, status, &response.ErrorResponse{Code: "REQUEST_FAILED", Message: err.Error()})
}

func handlePayInvoice(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invoiceID, ok := parsePathID(w, r, "invoiceID")
		if !ok {
			return
		}

		var input struct {
			ProviderCode   string `json:"provider_code"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if !decodeRequest(w, r, &input) {
			return
		}
		if input.ProviderCode == "" || input.IdempotencyKey == "" {
			writeAPIError(w, errors.New("provider_code and idempotency_key are required"))
			return
		}

		invoice, err := deps.InvoiceService.Get(r.Context(), invoiceID)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		result, err := deps.PPIService.ChargePaymentMethod(
			r.Context(), input.ProviderCode, invoiceID, invoice.AmountDue, input.IdempotencyKey,
		)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		response.Write(w, http.StatusOK, map[string]any{
			"status":             result.Status,
			"internal_tx_id":     result.IdempotencyKey,
			"provider_reference": result.ProviderReference,
			"checkout_url":       result.CheckoutURL,
			"failure_code":       result.FailureCode,
			"raw_response":       result.RawResponse,
		})
	}
}
