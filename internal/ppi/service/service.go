package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	attemptmodel "github.com/thec1oud/billing/internal/payment_attempt/model"
	attemptsrv "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/ppi/repository"
	"github.com/thec1oud/billing/internal/shared/money"
)

var (
	ErrDuplicateWebhook      = errors.New("duplicate webhook event")
	ErrUnsupportedProvider   = errors.New("unsupported payment provider")
	ErrProviderAdapterFailed = errors.New("provider adapter failed")
)

type Service struct {
	db         ppi.DBTX
	repo       *repository.PostgresRepository
	attemptSvc *attemptsrv.Service
	adapters   map[string]ppi.Provider
}

func NewService(
	db ppi.DBTX,
	repo *repository.PostgresRepository,
	attemptSvc *attemptsrv.Service,
) *Service {
	return &Service{
		db:         db,
		repo:       repo,
		attemptSvc: attemptSvc,
		adapters:   make(map[string]ppi.Provider),
	}
}

func (s *Service) RegisterAdapter(adapter ppi.Provider) {
	if adapter != nil {
		s.adapters[strings.ToLower(adapter.ProviderCode())] = adapter
	}
}

func (s *Service) GetAdapter(providerCode string) (ppi.Provider, bool) {
	adapter, exists := s.adapters[strings.ToLower(providerCode)]
	return adapter, exists
}

func (s *Service) ChargePaymentMethod(
	ctx context.Context,
	providerCode string,
	invoiceID int64,
	amount money.Money,
	idempotencyKey string,
) (ppi.ChargeResult, error) {
	adapter, exists := s.adapters[strings.ToLower(providerCode)]
	if !exists {
		return ppi.ChargeResult{}, fmt.Errorf("%w: %s", ErrUnsupportedProvider, providerCode)
	}

	// 1. Begin Database Transaction to record PENDING attempt
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return ppi.ChargeResult{}, fmt.Errorf("failed to begin transaction for charge: %w", err)
	}

	// 2. Create initial PENDING PaymentAttempt in DB transaction BEFORE calling provider
	rawReq, _ := json.Marshal(map[string]any{
		"idempotency_key": idempotencyKey,
		"provider_code":   providerCode,
	})
	attempt, err := s.attemptSvc.CreatePaymentAttempt(ctx, tx, attemptmodel.CreatePaymentAttemptInput{
		InvoiceID:    invoiceID,
		ProviderCode: providerCode,
		InternalTxID: idempotencyKey,
		Amount:       amount,
		RawRequest:   rawReq,
	})
	if err != nil {
		tx.Rollback(ctx)
		return ppi.ChargeResult{}, fmt.Errorf("failed to create payment attempt record in tx: %w", err)
	}

	// 3. Commit DB Transaction so it's visible to any synchronous webhooks
	if err := tx.Commit(ctx); err != nil {
		return ppi.ChargeResult{}, fmt.Errorf("failed to commit pending payment attempt transaction: %w", err)
	}

	// 4. Send request to provider through adapter
	result, adapterErr := adapter.ChargePaymentMethod(ctx, amount, providerCode, idempotencyKey)

	// 5. Begin a new transaction to update the attempt result (always needed,
	updateTx, err := s.db.Begin(ctx)
	if err != nil {
		return ppi.ChargeResult{}, fmt.Errorf("failed to begin update transaction: %w", err)
	}
	defer updateTx.Rollback(ctx)

	// 6. Handle provider failure: mark attempt as FAILED and surface the error
	if adapterErr != nil || result.Status == ppi.ChargeStatusFailed {
		_ = s.attemptSvc.UpdatePaymentAttemptResult(ctx, updateTx, attemptmodel.UpdatePaymentAttemptResultInput{
			AttemptID:   attempt.AttemptID,
			Status:      attemptmodel.StatusFailed,
			RawResponse: result.RawResponse,
		})
		_ = updateTx.Commit(ctx)

		if adapterErr != nil {
			return ppi.ChargeResult{}, fmt.Errorf("%w: %v", ErrProviderAdapterFailed, adapterErr)
		}
		return result, nil
	}

	// 6. Update attempt record in tx and commit
	var provRef *string
	if result.ProviderReference != "" {
		ref := result.ProviderReference
		provRef = &ref
	}

	err = s.attemptSvc.UpdatePaymentAttemptResult(ctx, updateTx, attemptmodel.UpdatePaymentAttemptResultInput{
		AttemptID:    attempt.AttemptID,
		Status:       attemptmodel.StatusSuccess,
		ProviderTxID: provRef,
		RawResponse:  result.RawResponse,
	})
	if err != nil {
		return ppi.ChargeResult{}, fmt.Errorf("failed to update payment attempt result in tx: %w", err)
	}

	// 7. Commit Update DB Transaction
	if err := updateTx.Commit(ctx); err != nil {
		return ppi.ChargeResult{}, fmt.Errorf("failed to commit payment attempt update transaction: %w", err)
	}

	return result, nil
}

func (s *Service) ProcessWebhook(ctx context.Context, payload ppi.ProviderWebhookPayload) (bool, error) {
	if s.repo == nil {
		return false, nil
	}

	// 1. Idempotency Check
	isDup, isPublished, err := s.repo.CheckWebhookStatus(ctx, s.db, payload.ProviderCode, payload.ProviderTxID)
	if err != nil {
		return false, fmt.Errorf("idempotency check query failed: %w", err)
	}
	if isDup {
		if isPublished {
			return true, nil
		}
		return false, nil
	}

	// 2. Save Webhook Audit Record in DB
	if err := s.repo.SaveWebhook(ctx, s.db, payload); err != nil {
		return false, fmt.Errorf("save webhook record failed: %w", err)
	}

	return false, nil
}

func (s *Service) MarkWebhookPublished(ctx context.Context, webhookID string) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.MarkPublished(ctx, s.db, webhookID)
}

var _ ppi.PPI = (*Service)(nil)
