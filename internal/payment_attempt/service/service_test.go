package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/payment_attempt/model"
	"github.com/thec1oud/billing/internal/payment_attempt/repository"
	"github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/shared/money"
)

type mockPaymentRepo struct {
	attemptsByTxID    map[string]model.PaymentAttempt
	pendingByInvoice  map[int64]model.PaymentAttempt
	createdAttempts   []model.PaymentAttempt
	createErr         error
	getByTxIDErr      error
	getPendingErr     error
	updateResultCalls int
}

func newMockPaymentRepo() *mockPaymentRepo {
	return &mockPaymentRepo{
		attemptsByTxID:   make(map[string]model.PaymentAttempt),
		pendingByInvoice: make(map[int64]model.PaymentAttempt),
	}
}

func (m *mockPaymentRepo) Create(
	ctx context.Context,
	db repository.DBTX,
	attempt model.PaymentAttempt,
) (model.PaymentAttempt, error) {
	if m.createErr != nil {
		return model.PaymentAttempt{}, m.createErr
	}
	attempt.AttemptID = int64(len(m.createdAttempts) + 1)
	attempt.CreatedAt = time.Now()
	attempt.UpdatedAt = time.Now()
	m.createdAttempts = append(m.createdAttempts, attempt)
	m.attemptsByTxID[attempt.InternalTxID] = attempt
	if attempt.Status == model.StatusPending {
		m.pendingByInvoice[attempt.InvoiceID] = attempt
	}
	return attempt, nil
}

func (m *mockPaymentRepo) GetByID(
	ctx context.Context,
	db repository.DBTX,
	attemptID int64,
) (model.PaymentAttempt, error) {
	return model.PaymentAttempt{}, repository.ErrPaymentAttemptNotFound
}

func (m *mockPaymentRepo) GetByInternalTxID(
	ctx context.Context,
	db repository.DBTX,
	internalTxID string,
) (model.PaymentAttempt, error) {
	if m.getByTxIDErr != nil {
		return model.PaymentAttempt{}, m.getByTxIDErr
	}
	att, exists := m.attemptsByTxID[internalTxID]
	if !exists {
		return model.PaymentAttempt{}, repository.ErrPaymentAttemptNotFound
	}
	return att, nil
}

func (m *mockPaymentRepo) GetByProviderTxID(
	ctx context.Context,
	db repository.DBTX,
	providerTxID string,
) (model.PaymentAttempt, error) {
	return model.PaymentAttempt{}, repository.ErrPaymentAttemptNotFound
}

func (m *mockPaymentRepo) GetPendingByInvoiceID(
	ctx context.Context,
	db repository.DBTX,
	invoiceID int64,
) (model.PaymentAttempt, error) {
	if m.getPendingErr != nil {
		return model.PaymentAttempt{}, m.getPendingErr
	}
	att, exists := m.pendingByInvoice[invoiceID]
	if !exists {
		return model.PaymentAttempt{}, repository.ErrPaymentAttemptNotFound
	}
	return att, nil
}

func (m *mockPaymentRepo) UpdateResult(
	ctx context.Context,
	db repository.DBTX,
	attemptID int64,
	status model.Status,
	providerTxID *string,
	rawResponse json.RawMessage,
) error {
	m.updateResultCalls++
	return nil
}

func (m *mockPaymentRepo) ListByInvoiceID(
	ctx context.Context,
	db repository.DBTX,
	invoiceID int64,
) ([]model.PaymentAttempt, error) {
	return nil, nil
}

func (m *mockPaymentRepo) DeleteByAttemptID(
	ctx context.Context,
	db repository.DBTX,
	attemptID int64,
) error {
	return nil
}

func TestCreatePaymentAttempt_Success(t *testing.T) {
	repo := newMockPaymentRepo()
	svc := service.NewService(repo)
	amt, _ := money.New(1000, "USD")

	input := model.CreatePaymentAttemptInput{
		InvoiceID:    1,
		ProviderCode: "fake",
		InternalTxID: "tx_1",
		Amount:       amt,
	}

	attempt, created, err := svc.CreatePaymentAttempt(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Fatalf("expected created to be true")
	}
	if attempt.AttemptID != 1 {
		t.Fatalf("expected attempt ID 1, got %d", attempt.AttemptID)
	}
	if attempt.Status != model.StatusPending {
		t.Fatalf("expected status PENDING, got %s", attempt.Status)
	}
}

func TestCreatePaymentAttempt_ReplayExisting(t *testing.T) {
	repo := newMockPaymentRepo()
	amt, _ := money.New(1000, "USD")
	provRef := "ref_success_123"
	repo.attemptsByTxID["tx_replay"] = model.PaymentAttempt{
		AttemptID:    99,
		InvoiceID:    1,
		ProviderCode: "fake",
		InternalTxID: "tx_replay",
		ProviderTxID: &provRef,
		Amount:       amt,
		Status:       model.StatusSuccess,
	}

	svc := service.NewService(repo)
	input := model.CreatePaymentAttemptInput{
		InvoiceID:    1,
		ProviderCode: "fake",
		InternalTxID: "tx_replay",
		Amount:       amt,
	}

	attempt, created, err := svc.CreatePaymentAttempt(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Fatalf("expected created to be false for replay")
	}
	if attempt.AttemptID != 99 {
		t.Fatalf("expected existing attempt ID 99, got %d", attempt.AttemptID)
	}
	if attempt.Status != model.StatusSuccess {
		t.Fatalf("expected status SUCCESS, got %s", attempt.Status)
	}
}

func TestCreatePaymentAttempt_PendingAttemptAlreadyExists(t *testing.T) {
	repo := newMockPaymentRepo()
	amt, _ := money.New(1000, "USD")
	repo.pendingByInvoice[1] = model.PaymentAttempt{
		AttemptID:    10,
		InvoiceID:    1,
		InternalTxID: "tx_in_flight",
		Status:       model.StatusPending,
		Amount:       amt,
	}

	svc := service.NewService(repo)
	input := model.CreatePaymentAttemptInput{
		InvoiceID:    1,
		ProviderCode: "fake",
		InternalTxID: "tx_second_key",
		Amount:       amt,
	}

	_, created, err := svc.CreatePaymentAttempt(context.Background(), nil, input)
	if !errors.Is(err, repository.ErrPendingAttemptExists) {
		t.Fatalf("expected ErrPendingAttemptExists, got %v", err)
	}
	if created {
		t.Fatalf("expected created to be false")
	}
}

func TestCreatePaymentAttempt_UniqueConstraintConflictFromRepo(t *testing.T) {
	repo := newMockPaymentRepo()
	repo.createErr = repository.ErrPendingAttemptExists
	svc := service.NewService(repo)
	amt, _ := money.New(1000, "USD")

	input := model.CreatePaymentAttemptInput{
		InvoiceID:    1,
		ProviderCode: "fake",
		InternalTxID: "tx_race",
		Amount:       amt,
	}

	_, created, err := svc.CreatePaymentAttempt(context.Background(), nil, input)
	if !errors.Is(err, repository.ErrPendingAttemptExists) {
		t.Fatalf("expected ErrPendingAttemptExists, got %v", err)
	}
	if created {
		t.Fatalf("expected created to be false")
	}
}
