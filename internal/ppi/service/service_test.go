package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	attemptmodel "github.com/thec1oud/billing/internal/payment_attempt/model"
	attemptrepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptsrv "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/ppi/service"
	"github.com/thec1oud/billing/internal/shared/money"
)

type mockTx struct {
	pgx.Tx
	committed  bool
	rolledBack bool
}

func (m *mockTx) Commit(ctx context.Context) error {
	m.committed = true
	return nil
}

func (m *mockTx) Rollback(ctx context.Context) error {
	m.rolledBack = true
	return nil
}

type mockDB struct {
	ppi.DBTX
	txs []*mockTx
}

func (m *mockDB) Begin(ctx context.Context) (pgx.Tx, error) {
	tx := &mockTx{}
	m.txs = append(m.txs, tx)
	return tx, nil
}

type mockPaymentRepo struct {
	attemptsByTxID   map[string]attemptmodel.PaymentAttempt
	pendingByInvoice map[int64]attemptmodel.PaymentAttempt
	createdAttempts  []attemptmodel.PaymentAttempt
	createErr        error
	updateCalls      int
}

func newMockPaymentRepo() *mockPaymentRepo {
	return &mockPaymentRepo{
		attemptsByTxID:   make(map[string]attemptmodel.PaymentAttempt),
		pendingByInvoice: make(map[int64]attemptmodel.PaymentAttempt),
	}
}

func (m *mockPaymentRepo) Create(
	ctx context.Context,
	db attemptrepo.DBTX,
	attempt attemptmodel.PaymentAttempt,
) (attemptmodel.PaymentAttempt, error) {
	if m.createErr != nil {
		return attemptmodel.PaymentAttempt{}, m.createErr
	}
	attempt.AttemptID = int64(len(m.createdAttempts) + 1)
	attempt.CreatedAt = time.Now()
	attempt.UpdatedAt = time.Now()
	m.createdAttempts = append(m.createdAttempts, attempt)
	m.attemptsByTxID[attempt.InternalTxID] = attempt
	if attempt.Status == attemptmodel.StatusPending {
		m.pendingByInvoice[attempt.InvoiceID] = attempt
	}
	return attempt, nil
}

func (m *mockPaymentRepo) GetByID(
	ctx context.Context,
	db attemptrepo.DBTX,
	attemptID int64,
) (attemptmodel.PaymentAttempt, error) {
	return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPaymentAttemptNotFound
}

func (m *mockPaymentRepo) GetByInternalTxID(
	ctx context.Context,
	db attemptrepo.DBTX,
	internalTxID string,
) (attemptmodel.PaymentAttempt, error) {
	att, exists := m.attemptsByTxID[internalTxID]
	if !exists {
		return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPaymentAttemptNotFound
	}
	return att, nil
}

func (m *mockPaymentRepo) GetByProviderTxID(
	ctx context.Context,
	db attemptrepo.DBTX,
	providerTxID string,
) (attemptmodel.PaymentAttempt, error) {
	return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPaymentAttemptNotFound
}

func (m *mockPaymentRepo) GetPendingByInvoiceID(
	ctx context.Context,
	db attemptrepo.DBTX,
	invoiceID int64,
) (attemptmodel.PaymentAttempt, error) {
	att, exists := m.pendingByInvoice[invoiceID]
	if !exists {
		return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPaymentAttemptNotFound
	}
	return att, nil
}

func (m *mockPaymentRepo) UpdateResult(
	ctx context.Context,
	db attemptrepo.DBTX,
	attemptID int64,
	status attemptmodel.Status,
	providerTxID *string,
	rawResponse json.RawMessage,
) error {
	m.updateCalls++
	for k, att := range m.attemptsByTxID {
		if att.AttemptID == attemptID {
			if att.Status != attemptmodel.StatusPending {
				return attemptrepo.ErrAttemptAlreadyFinalized
			}
			att.Status = status
			att.ProviderTxID = providerTxID
			att.RawResponse = rawResponse
			m.attemptsByTxID[k] = att
			if status != attemptmodel.StatusPending {
				delete(m.pendingByInvoice, att.InvoiceID)
			}
			return nil
		}
	}
	return attemptrepo.ErrAttemptAlreadyFinalized
}

func (m *mockPaymentRepo) ListByInvoiceID(
	ctx context.Context,
	db attemptrepo.DBTX,
	invoiceID int64,
) ([]attemptmodel.PaymentAttempt, error) {
	return nil, nil
}

func (m *mockPaymentRepo) DeleteByAttemptID(
	ctx context.Context,
	db attemptrepo.DBTX,
	attemptID int64,
) error {
	return nil
}

type mockProvider struct {
	code         string
	chargeCalls  int
	chargeResult ppi.ChargeResult
	chargeErr    error
	onCharge     func()
}

func (m *mockProvider) ProviderCode() string {
	return m.code
}

func (m *mockProvider) ChargePaymentMethod(
	ctx context.Context,
	amount money.Money,
	providerCode, idempotencyKey string,
) (ppi.ChargeResult, error) {
	m.chargeCalls++
	if m.onCharge != nil {
		m.onCharge()
	}
	return m.chargeResult, m.chargeErr
}

func (m *mockProvider) VerifyPayment(
	ctx context.Context,
	providerCode, internalTxID string,
	providerTxID *string,
) (ppi.ChargeResult, error) {
	return ppi.ChargeResult{}, nil
}

func (m *mockProvider) ParseWebhook(r *http.Request) (ppi.ProviderWebhookPayload, error) {
	return ppi.ProviderWebhookPayload{}, nil
}

func (m *mockProvider) RespondWebhook(
	w http.ResponseWriter,
	r *http.Request,
	code ppi.WebhookResponseCode,
	payload ppi.ProviderWebhookPayload,
) {
}

func TestChargePaymentMethod_IdempotencyReplayMakesNoSecondProviderCall(t *testing.T) {
	db := &mockDB{}
	paymentRepo := newMockPaymentRepo()
	attemptService := attemptsrv.NewService(paymentRepo)

	provider := &mockProvider{
		code: "mock_pay",
		chargeResult: ppi.ChargeResult{
			Status:            ppi.ChargeStatusSuccess,
			ProviderReference: "prov_ref_success_001",
		},
	}

	svc := service.NewService(db, nil, attemptService, nil)
	svc.RegisterAdapter(provider)

	amt, _ := money.New(5000, "USD")
	idempotencyKey := "idem_key_12345"

	// 1. First charge: should call provider
	res1, err := svc.ChargePaymentMethod(context.Background(), "mock_pay", 100, amt, idempotencyKey)
	if err != nil {
		t.Fatalf("unexpected error on first charge: %v", err)
	}
	if res1.Status != ppi.ChargeStatusSuccess {
		t.Fatalf("expected status SUCCESS, got %s", res1.Status)
	}
	if provider.chargeCalls != 1 {
		t.Fatalf("expected exactly 1 provider call on first attempt, got %d", provider.chargeCalls)
	}

	// 2. Second charge with identical idempotency key: MUST NOT call provider again
	res2, err := svc.ChargePaymentMethod(context.Background(), "mock_pay", 100, amt, idempotencyKey)
	if err != nil {
		t.Fatalf("unexpected error on replay charge: %v", err)
	}
	if res2.Status != ppi.ChargeStatusSuccess {
		t.Fatalf("expected status SUCCESS on replay, got %s", res2.Status)
	}
	if res2.ProviderReference != "prov_ref_success_001" {
		t.Fatalf("expected preserved provider reference 'prov_ref_success_001', got %s", res2.ProviderReference)
	}
	if provider.chargeCalls != 1 {
		t.Fatalf("expected provider.chargeCalls to remain 1 on idempotency replay, got %d", provider.chargeCalls)
	}

	// 3. Replay with an attempt whose status is FAILED
	paymentRepo.attemptsByTxID["tx_failed_prev"] = attemptmodel.PaymentAttempt{
		AttemptID:    99,
		InvoiceID:    101,
		ProviderCode: "mock_pay",
		InternalTxID: "tx_failed_prev",
		Status:       attemptmodel.StatusFailed,
		Amount:       amt,
	}
	resFailed, err := svc.ChargePaymentMethod(context.Background(), "mock_pay", 101, amt, "tx_failed_prev")
	if err != nil {
		t.Fatalf("unexpected error on failed replay: %v", err)
	}
	if resFailed.Status != ppi.ChargeStatusFailed {
		t.Fatalf("expected status FAILED, got %s", resFailed.Status)
	}
	if provider.chargeCalls != 1 {
		t.Fatalf("provider should still not be called for replay of failed attempt, got %d", provider.chargeCalls)
	}

	// 4. Replay with an attempt whose status is PENDING
	paymentRepo.attemptsByTxID["tx_pending_prev"] = attemptmodel.PaymentAttempt{
		AttemptID:    100,
		InvoiceID:    102,
		ProviderCode: "mock_pay",
		InternalTxID: "tx_pending_prev",
		Status:       attemptmodel.StatusPending,
		Amount:       amt,
	}
	resPending, err := svc.ChargePaymentMethod(context.Background(), "mock_pay", 102, amt, "tx_pending_prev")
	if err != nil {
		t.Fatalf("unexpected error on pending replay: %v", err)
	}
	if resPending.Status != ppi.ChargeStatusPending {
		t.Fatalf("expected status PENDING, got %s", resPending.Status)
	}
	if provider.chargeCalls != 1 {
		t.Fatalf("provider should still not be called for replay of pending attempt, got %d", provider.chargeCalls)
	}
}

func TestChargePaymentMethod_ConcurrentPaysOnOneInvoiceProduceExactlyOneProviderCharge(t *testing.T) {
	db := &mockDB{}
	paymentRepo := newMockPaymentRepo()
	attemptService := attemptsrv.NewService(paymentRepo)

	provider := &mockProvider{
		code: "mock_pay",
		chargeResult: ppi.ChargeResult{
			Status:            ppi.ChargeStatusSuccess,
			ProviderReference: "prov_ref_002",
		},
	}

	svc := service.NewService(db, nil, attemptService, nil)
	svc.RegisterAdapter(provider)

	amt, _ := money.New(5000, "USD")

	// Pre-populate an in-flight (PENDING) payment attempt for invoice 200
	paymentRepo.pendingByInvoice[200] = attemptmodel.PaymentAttempt{
		AttemptID:    1,
		InvoiceID:    200,
		ProviderCode: "mock_pay",
		InternalTxID: "tx_first_in_flight",
		Status:       attemptmodel.StatusPending,
		Amount:       amt,
	}
	paymentRepo.attemptsByTxID["tx_first_in_flight"] = paymentRepo.pendingByInvoice[200]

	// Now try to initiate a concurrent charge on the same invoice with a different idempotency key
	_, err := svc.ChargePaymentMethod(context.Background(), "mock_pay", 200, amt, "tx_second_concurrent")
	if err == nil {
		t.Fatalf("expected error for concurrent payment on same invoice, got nil")
	}
	if !errors.Is(err, attemptrepo.ErrPendingAttemptExists) {
		t.Fatalf("expected error to wrap ErrPendingAttemptExists, got %v", err)
	}

	// Verify provider was NEVER called for the second concurrent request
	if provider.chargeCalls != 0 {
		t.Fatalf("expected 0 provider calls for concurrent charge, got %d", provider.chargeCalls)
	}
}

func TestChargePaymentMethod_IdempotencyKeyMismatchRejected(t *testing.T) {
	db := &mockDB{}
	paymentRepo := newMockPaymentRepo()
	attemptService := attemptsrv.NewService(paymentRepo)
	provider := &mockProvider{code: "mock_pay"}

	svc := service.NewService(db, nil, attemptService, nil)
	svc.RegisterAdapter(provider)

	amt, _ := money.New(5000, "USD")
	paymentRepo.attemptsByTxID["tx_invoice_1"] = attemptmodel.PaymentAttempt{
		AttemptID:    1,
		InvoiceID:    100,
		ProviderCode: "mock_pay",
		InternalTxID: "tx_invoice_1",
		Status:       attemptmodel.StatusSuccess,
		Amount:       amt,
	}

	_, err := svc.ChargePaymentMethod(context.Background(), "mock_pay", 999, amt, "tx_invoice_1")
	if !errors.Is(err, attemptrepo.ErrIdempotencyKeyMismatch) {
		t.Fatalf("expected ErrIdempotencyKeyMismatch, got %v", err)
	}
	if provider.chargeCalls != 0 {
		t.Fatalf("expected 0 provider calls on key mismatch, got %d", provider.chargeCalls)
	}
}

func TestChargePaymentMethod_ConcurrentDuplicateKeyReplaysStoredResult(t *testing.T) {
	db := &mockDB{}
	paymentRepo := newMockPaymentRepo()
	attemptService := attemptsrv.NewService(paymentRepo)
	provider := &mockProvider{code: "mock_pay"}

	svc := service.NewService(db, nil, attemptService, nil)
	svc.RegisterAdapter(provider)

	amt, _ := money.New(5000, "USD")
	provRef := "prov_concurrent_ok"
	// Simulate Create failing due to a concurrent winner inserting the same key first.
	paymentRepo.createErr = attemptrepo.ErrDuplicateInternalTxID

	// Inject a custom repo wrapper or seed after GetByInternalTxID misses by checking fallback:
	// Use a repo that fails Create with ErrDuplicateInternalTxID and exposes the winner on subsequent GetByInternalTxID.
	raceRepo := &raceCreateDuplicateRepo{
		mockPaymentRepo: paymentRepo,
		winner: attemptmodel.PaymentAttempt{
			AttemptID:    42,
			InvoiceID:    100,
			ProviderCode: "mock_pay",
			InternalTxID: "tx_race_same_key",
			ProviderTxID: &provRef,
			Status:       attemptmodel.StatusSuccess,
			Amount:       amt,
		},
	}
	raceSvc := service.NewService(db, nil, attemptsrv.NewService(raceRepo), nil)
	raceSvc.RegisterAdapter(provider)

	res, err := raceSvc.ChargePaymentMethod(context.Background(), "mock_pay", 100, amt, "tx_race_same_key")
	if err != nil {
		t.Fatalf("unexpected error on concurrent same-key replay: %v", err)
	}
	if res.Status != ppi.ChargeStatusSuccess || res.ProviderReference != provRef {
		t.Fatalf("unexpected replay result: %+v", res)
	}
	if provider.chargeCalls != 0 {
		t.Fatalf("expected 0 provider calls on concurrent duplicate replay, got %d", provider.chargeCalls)
	}
}

type raceCreateDuplicateRepo struct {
	*mockPaymentRepo
	winner attemptmodel.PaymentAttempt
	calls  int
}

func (r *raceCreateDuplicateRepo) GetByInternalTxID(
	ctx context.Context,
	db attemptrepo.DBTX,
	internalTxID string,
) (attemptmodel.PaymentAttempt, error) {
	r.calls++
	if r.calls == 1 {
		return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPaymentAttemptNotFound
	}
	return r.winner, nil
}

func TestChargePaymentMethod_FinalizedConcurrentlyReturnsAuthoritativeDBResult(t *testing.T) {
	amt, _ := money.New(5000, "USD")

	t.Run("webhook marks SUCCESS while adapter returns error", func(t *testing.T) {
		db := &mockDB{}
		paymentRepo := newMockPaymentRepo()
		attemptService := attemptsrv.NewService(paymentRepo)

		webhookRef := "prov_from_webhook"
		provider := &mockProvider{
			code:      "mock_pay",
			chargeErr: errors.New("http timeout reading provider response"),
			onCharge: func() {
				att := paymentRepo.attemptsByTxID["tx_race_finalize"]
				att.Status = attemptmodel.StatusSuccess
				att.ProviderTxID = &webhookRef
				paymentRepo.attemptsByTxID["tx_race_finalize"] = att
			},
		}

		svc := service.NewService(db, nil, attemptService, nil)
		svc.RegisterAdapter(provider)

		res, err := svc.ChargePaymentMethod(context.Background(), "mock_pay", 100, amt, "tx_race_finalize")
		if err != nil {
			t.Fatalf("expected authoritative DB result with nil error, got %v", err)
		}
		if res.Status != ppi.ChargeStatusSuccess {
			t.Fatalf("expected SUCCESS from DB, got %s", res.Status)
		}
		if res.ProviderReference != webhookRef {
			t.Fatalf("expected provider ref %q, got %q", webhookRef, res.ProviderReference)
		}
	})

	t.Run("reconciler marks FAILED while adapter succeeds locally", func(t *testing.T) {
		db := &mockDB{}
		paymentRepo := newMockPaymentRepo()
		attemptService := attemptsrv.NewService(paymentRepo)

		provider := &mockProvider{
			code: "mock_pay",
			chargeResult: ppi.ChargeResult{
				Status:            ppi.ChargeStatusSuccess,
				ProviderReference: "stale_ref",
			},
			onCharge: func() {
				att := paymentRepo.attemptsByTxID["tx_race_reconciler"]
				att.Status = attemptmodel.StatusFailed
				paymentRepo.attemptsByTxID["tx_race_reconciler"] = att
			},
		}

		svc := service.NewService(db, nil, attemptService, nil)
		svc.RegisterAdapter(provider)

		res, err := svc.ChargePaymentMethod(context.Background(), "mock_pay", 101, amt, "tx_race_reconciler")
		if err != nil {
			t.Fatalf("expected authoritative DB result with nil error, got %v", err)
		}
		if res.Status != ppi.ChargeStatusFailed {
			t.Fatalf("expected FAILED from DB, got %s", res.Status)
		}
	})
}

func TestLookupCharge(t *testing.T) {
	db := &mockDB{}
	paymentRepo := newMockPaymentRepo()
	attemptService := attemptsrv.NewService(paymentRepo)
	svc := service.NewService(db, nil, attemptService, nil)

	amt, _ := money.New(5000, "USD")
	provRef := "prov_lookup_1"
	paymentRepo.attemptsByTxID["tx_lookup"] = attemptmodel.PaymentAttempt{
		AttemptID:    7,
		InvoiceID:    300,
		ProviderCode: "mock_pay",
		InternalTxID: "tx_lookup",
		ProviderTxID: &provRef,
		Status:       attemptmodel.StatusSuccess,
		Amount:       amt,
	}

	res, found, err := svc.LookupCharge(context.Background(), "mock_pay", 300, "tx_lookup")
	if err != nil || !found {
		t.Fatalf("expected found=true, err=nil; got found=%v err=%v", found, err)
	}
	if res.Status != ppi.ChargeStatusSuccess || res.ProviderReference != provRef {
		t.Fatalf("unexpected lookup result: %+v", res)
	}

	_, foundMissing, err := svc.LookupCharge(context.Background(), "mock_pay", 300, "tx_missing")
	if err != nil || foundMissing {
		t.Fatalf("expected found=false, err=nil; got found=%v err=%v", foundMissing, err)
	}

	_, foundMismatch, err := svc.LookupCharge(context.Background(), "mock_pay", 999, "tx_lookup")
	if !errors.Is(err, attemptrepo.ErrIdempotencyKeyMismatch) || foundMismatch {
		t.Fatalf("expected ErrIdempotencyKeyMismatch, got found=%v err=%v", foundMismatch, err)
	}
}
