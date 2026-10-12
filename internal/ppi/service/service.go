package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

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

type OutboxEnqueuerFunc func(ctx context.Context, tx ppi.DBTX, actionName string, params json.RawMessage) error

type Service struct {
	db             ppi.DBTX
	repo           *repository.PostgresRepository
	attemptSvc     *attemptsrv.Service
	outboxEnqueuer OutboxEnqueuerFunc
	adapters       map[string]ppi.Provider
}

func NewService(
	db ppi.DBTX,
	repo *repository.PostgresRepository,
	attemptSvc *attemptsrv.Service,
	outboxEnqueuer OutboxEnqueuerFunc,
) *Service {
	return &Service{
		db:             db,
		repo:           repo,
		attemptSvc:     attemptSvc,
		outboxEnqueuer: outboxEnqueuer,
		adapters:       make(map[string]ppi.Provider),
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
	defer tx.Rollback(ctx) //nolint:errcheck

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
		updateErr := s.attemptSvc.UpdatePaymentAttemptResult(ctx, updateTx, attemptmodel.UpdatePaymentAttemptResultInput{
			AttemptID:   attempt.AttemptID,
			Status:      attemptmodel.StatusFailed,
			RawResponse: result.RawResponse,
		})

		var commitErr error
		if updateErr == nil {
			commitErr = updateTx.Commit(ctx)
		}

		if adapterErr != nil {
			if updateErr != nil {
				return ppi.ChargeResult{}, fmt.Errorf(
					"%w: %v (also failed to update attempt: %v)",
					ErrProviderAdapterFailed, adapterErr, updateErr,
				)
			}
			if commitErr != nil {
				return ppi.ChargeResult{}, fmt.Errorf(
					"%w: %v (also failed to commit attempt update: %v)",
					ErrProviderAdapterFailed, adapterErr, commitErr,
				)
			}
			return ppi.ChargeResult{}, fmt.Errorf("%w: %v", ErrProviderAdapterFailed, adapterErr)
		}

		if updateErr != nil {
			return result, fmt.Errorf("failed to update payment attempt result to failed: %w", updateErr)
		}
		if commitErr != nil {
			return result, fmt.Errorf("failed to commit failed payment attempt update: %w", commitErr)
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
		return result, fmt.Errorf("provider charge succeeded but failed to update attempt record in tx (needs reconciliation): %w", err)
	}

	// 7. Commit Update DB Transaction
	if err := updateTx.Commit(ctx); err != nil {
		return result, fmt.Errorf("provider charge succeeded but failed to commit attempt update transaction (needs reconciliation): %w", err)
	}

	return result, nil
}

func (s *Service) ProcessWebhook(ctx context.Context, payload ppi.ProviderWebhookPayload) (bool, error) {
	if s.repo == nil {
		return false, nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. Atomic Save Webhook Audit Record in DB
	err = s.repo.SaveWebhook(ctx, tx, payload)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Duplicate found (ON CONFLICT DO NOTHING returned no rows)
			isPublished, checkErr := s.repo.GetWebhookPublishedStatus(ctx, tx, payload.ProviderCode, payload.ProviderTxID)
			if checkErr != nil {
				return false, fmt.Errorf("idempotency check query failed for duplicate webhook: %w", checkErr)
			}
			return isPublished, nil
		}
		return false, fmt.Errorf("save webhook record failed: %w", err)
	}

	// 2. Enqueue Outbox row for RabbitMQ publisher
	if s.outboxEnqueuer != nil {
		routingKey := DetermineRoutingKey(payload)
		msgBytes, err := json.Marshal(payload)
		if err != nil {
			return false, fmt.Errorf("failed to marshal webhook payload: %w", err)
		}
		outboxPayload := struct {
			RoutingKey string          `json:"routing_key"`
			Payload    json.RawMessage `json:"payload"`
		}{
			RoutingKey: routingKey,
			Payload:    json.RawMessage(msgBytes),
		}
		params, err := json.Marshal(outboxPayload)
		if err != nil {
			return false, fmt.Errorf("failed to marshal outbox payload: %w", err)
		}

		if err := s.outboxEnqueuer(ctx, tx, "PublishWebhook", params); err != nil {
			return false, fmt.Errorf("failed to enqueue outbox row: %w", err)
		}
	}

	// 3. Commit
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("failed to commit webhook transaction: %w", err)
	}

	return false, nil
}

func (s *Service) MarkWebhookPublished(ctx context.Context, webhookID string) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.MarkPublished(ctx, s.db, webhookID)
}

func DetermineRoutingKey(p ppi.ProviderWebhookPayload) string {
	switch p.Status {
	case ppi.WebhookPaymentSucceeded:
		return "ppi.webhook.payment.succeeded"
	case ppi.WebhookPaymentFailed:
		return "ppi.webhook.payment.failed"
	default:
		return "ppi.webhook.payment.pending"
	}
}

var _ ppi.PPI = (*Service)(nil)
