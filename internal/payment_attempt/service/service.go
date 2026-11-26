package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/thec1oud/billing/internal/payment_attempt/model"
	"github.com/thec1oud/billing/internal/payment_attempt/repository"
	"github.com/thec1oud/billing/internal/shared/money"
)

var (
	ErrInvoiceNotFound      = errors.New("invoice not found")
	ErrPaymentMethodInvalid = errors.New("invalid or inactive payment method")
)

type PaymentRepository interface {
	Create(ctx context.Context, db repository.DBTX, attempt model.PaymentAttempt) (model.PaymentAttempt, error)
	GetByID(ctx context.Context, db repository.DBTX, attemptID int64) (model.PaymentAttempt, error)
	GetByInternalTxID(ctx context.Context, db repository.DBTX, internalTxID string) (model.PaymentAttempt, error)
	GetByProviderTxID(ctx context.Context, db repository.DBTX, providerTxID string) (model.PaymentAttempt, error)
	GetPendingByInvoiceID(ctx context.Context, db repository.DBTX, invoiceID int64) (model.PaymentAttempt, error)
	UpdateResult(
		ctx context.Context,
		db repository.DBTX,
		attemptID int64,
		status model.Status,
		providerTxID *string,
		rawResponse json.RawMessage,
	) error
	ListByInvoiceID(ctx context.Context, db repository.DBTX, invoiceID int64) ([]model.PaymentAttempt, error)
	DeleteByAttemptID(ctx context.Context, db repository.DBTX, attemptID int64) error
}

type Service struct {
	repo PaymentRepository
}

func NewService(repo PaymentRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetAttemptByInternalTxID(
	ctx context.Context,
	db repository.DBTX,
	internalTxID string,
) (model.PaymentAttempt, error) {
	return s.repo.GetByInternalTxID(ctx, db, internalTxID)
}

// ValidateAttemptMatches ensures a replayed idempotency key uses the same invoice, provider, and amount.
func ValidateAttemptMatches(
	existing model.PaymentAttempt,
	invoiceID int64,
	providerCode string,
	amount money.Money,
) error {
	if existing.InvoiceID != invoiceID ||
		existing.Amount.AmountMinor != amount.AmountMinor ||
		existing.Amount.Currency != amount.Currency ||
		!strings.EqualFold(existing.ProviderCode, providerCode) {
		return fmt.Errorf(
			"%w: attempt invoice=%d provider=%s amount=%d %s",
			repository.ErrIdempotencyKeyMismatch,
			existing.InvoiceID,
			existing.ProviderCode,
			existing.Amount.AmountMinor,
			existing.Amount.Currency,
		)
	}
	return nil
}

// GetMatchingAttemptByInternalTxID loads an existing attempt by key and verifies its invoice, provider, and amount match.
func (s *Service) GetMatchingAttemptByInternalTxID(
	ctx context.Context,
	db repository.DBTX,
	invoiceID int64,
	providerCode string,
	amount money.Money,
	internalTxID string,
) (model.PaymentAttempt, error) {
	existing, err := s.repo.GetByInternalTxID(ctx, db, internalTxID)
	if err != nil {
		return model.PaymentAttempt{}, err
	}
	if err := ValidateAttemptMatches(existing, invoiceID, providerCode, amount); err != nil {
		return model.PaymentAttempt{}, err
	}
	return existing, nil
}

func (s *Service) CreatePaymentAttempt(
	ctx context.Context,
	db repository.DBTX,
	input model.CreatePaymentAttemptInput,
) (model.PaymentAttempt, bool, error) {
	existing, err := s.repo.GetByInternalTxID(ctx, db, input.InternalTxID)
	if err == nil {
		if matchErr := ValidateAttemptMatches(existing, input.InvoiceID, input.ProviderCode, input.Amount); matchErr != nil {
			return model.PaymentAttempt{}, false, matchErr
		}
		return existing, false, nil
	}
	if !errors.Is(err, repository.ErrPaymentAttemptNotFound) {
		return model.PaymentAttempt{}, false, fmt.Errorf("check internal tx id idempotency: %w", err)
	}

	pending, err := s.repo.GetPendingByInvoiceID(ctx, db, input.InvoiceID)
	if err == nil {
		return pending, false, repository.ErrPendingAttemptExists
	}
	if !errors.Is(err, repository.ErrPaymentAttemptNotFound) {
		return model.PaymentAttempt{}, false, fmt.Errorf("check pending attempt: %w", err)
	}

	attempt := model.PaymentAttempt{
		InvoiceID:    input.InvoiceID,
		ProviderCode: input.ProviderCode,
		InternalTxID: input.InternalTxID,
		Amount:       input.Amount,
		Status:       model.StatusPending,
		RawRequest:   input.RawRequest,
	}

	created, err := s.repo.Create(ctx, db, attempt)
	if err != nil {
		return model.PaymentAttempt{}, false, err
	}

	return created, true, nil
}

func (s *Service) UpdatePaymentAttemptResult(
	ctx context.Context,
	db repository.DBTX,
	input model.UpdatePaymentAttemptResultInput,
) error {
	err := s.repo.UpdateResult(
		ctx,
		db,
		input.AttemptID,
		input.Status,
		input.ProviderTxID,
		input.RawResponse,
	)
	if err != nil {
		return err
	}

	return nil
}
