package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/thec1oud/billing/internal/payment_attempt/model"
	"github.com/thec1oud/billing/internal/payment_attempt/repository"
)

var (
	ErrInvoiceNotFound      = errors.New("invoice not found")
	ErrPaymentMethodInvalid = errors.New("invalid or inactive payment method")
)

type PaymentRepository interface {
	Create(ctx context.Context, db repository.DBTX, attempt model.PaymentAttempt) (model.PaymentAttempt, error)
	GetNextAttemptNumber(ctx context.Context, db repository.DBTX, invoiceID int64) (int, error)
	GetByID(ctx context.Context, db repository.DBTX, id uuid.UUID) (model.PaymentAttempt, error)
	GetByIdempotencyKey(ctx context.Context, db repository.DBTX, key string) (model.PaymentAttempt, error)
	GetByProviderTxID(ctx context.Context, db repository.DBTX, providerTxID string) (model.PaymentAttempt, error)
	GetPendingByInvoiceID(ctx context.Context, db repository.DBTX, invoiceID int64) (model.PaymentAttempt, error)
	UpdateStatus(ctx context.Context, db repository.DBTX, id uuid.UUID, status model.Status, providerTxID *string, rawResponse json.RawMessage) error
	ListByInvoiceID(ctx context.Context, db repository.DBTX, invoiceID int64) ([]model.PaymentAttempt, error)
	ListByPaymentMethodID(ctx context.Context, db repository.DBTX, paymentMethodID int64) ([]model.PaymentAttempt, error)
}

type Service struct {
	repo PaymentRepository
}

func NewService(repo PaymentRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreatePaymentAttempt(
	ctx context.Context,
	db repository.DBTX,
	input model.CreatePaymentAttemptInput,
) (model.PaymentAttempt, error) {
	existing, err := s.repo.GetByIdempotencyKey(ctx, db, input.IdempotencyKey)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, repository.ErrPaymentAttemptNotFound) {
		return model.PaymentAttempt{}, fmt.Errorf("check idempotency key: %w", err)
	}

	_, err = s.repo.GetPendingByInvoiceID(ctx, db, input.InvoiceID)
	if err == nil {
		return model.PaymentAttempt{}, repository.ErrPendingAttemptExists
	}
	if !errors.Is(err, repository.ErrPaymentAttemptNotFound) {
		return model.PaymentAttempt{}, fmt.Errorf("check pending attempts: %w", err)
	}

	nextAttemptNum, err := s.repo.GetNextAttemptNumber(ctx, db, input.InvoiceID)
	if err != nil {
		return model.PaymentAttempt{}, fmt.Errorf("calculate next attempt number: %w", err)
	}

	attempt := model.PaymentAttempt{
		InvoiceID:       input.InvoiceID,
		PaymentMethodID: &input.PaymentMethodID,
		AttemptNumber:   nextAttemptNum,
		IdempotencyKey:  input.IdempotencyKey,
		ProviderTxID:    input.ProviderTxID,
		Amount:          input.Amount,
		Status:          model.StatusPending,
		RawResponse:     input.RawResponse,
	}

	created, err := s.repo.Create(ctx, db, attempt)
	if err != nil {
		return model.PaymentAttempt{}, fmt.Errorf("persist payment attempt: %w", err)
	}

	return created, nil
}
