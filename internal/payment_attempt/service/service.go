package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thec1oud/billing/internal/payment_attempt/model"
	"github.com/thec1oud/billing/internal/payment_attempt/repository"
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
	UpdateResult(ctx context.Context, db repository.DBTX, attemptID int64, status model.Status, providerTxID *string, rawResponse json.RawMessage) error
	ListByInvoiceID(ctx context.Context, db repository.DBTX, invoiceID int64) ([]model.PaymentAttempt, error)
	DeleteByAttemptID(ctx context.Context, db repository.DBTX, attemptID int64) error
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
	existing, err := s.repo.GetByInternalTxID(ctx, db, input.InternalTxID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, repository.ErrPaymentAttemptNotFound) {
		return model.PaymentAttempt{}, fmt.Errorf("check internal tx id idempotency: %w", err)
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
		return model.PaymentAttempt{}, err
	}

	return created, nil
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
