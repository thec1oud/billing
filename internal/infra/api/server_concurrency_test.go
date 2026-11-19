package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	invoicehandler "github.com/thec1oud/billing/internal/invoice/handler"
	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
	attemptmodel "github.com/thec1oud/billing/internal/payment_attempt/model"
	attemptrepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptservice "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	ppihandler "github.com/thec1oud/billing/internal/ppi/handler"
	ppiservice "github.com/thec1oud/billing/internal/ppi/service"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/money"
)

type threadSafeTx struct {
	pgx.Tx
}

func (t *threadSafeTx) Commit(ctx context.Context) error {
	return nil
}

func (t *threadSafeTx) Rollback(ctx context.Context) error {
	return nil
}

type threadSafeDB struct {
	ppi.DBTX
	mu  sync.Mutex
	txs []*threadSafeTx
}

func (m *threadSafeDB) Begin(ctx context.Context) (pgx.Tx, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tx := &threadSafeTx{}
	m.txs = append(m.txs, tx)
	return tx, nil
}

type threadSafePaymentRepo struct {
	mu               sync.Mutex
	attemptsByTxID   map[string]attemptmodel.PaymentAttempt
	pendingByInvoice map[int64]attemptmodel.PaymentAttempt
	createdAttempts  []attemptmodel.PaymentAttempt
}

func newThreadSafePaymentRepo() *threadSafePaymentRepo {
	return &threadSafePaymentRepo{
		attemptsByTxID:   make(map[string]attemptmodel.PaymentAttempt),
		pendingByInvoice: make(map[int64]attemptmodel.PaymentAttempt),
	}
}

func (r *threadSafePaymentRepo) Create(
	ctx context.Context,
	db attemptrepo.DBTX,
	attempt attemptmodel.PaymentAttempt,
) (attemptmodel.PaymentAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.pendingByInvoice[attempt.InvoiceID]; exists {
		return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPendingAttemptExists
	}

	attempt.AttemptID = int64(len(r.createdAttempts) + 1)
	attempt.CreatedAt = time.Now()
	attempt.UpdatedAt = time.Now()
	r.createdAttempts = append(r.createdAttempts, attempt)
	r.attemptsByTxID[attempt.InternalTxID] = attempt
	if attempt.Status == attemptmodel.StatusPending {
		r.pendingByInvoice[attempt.InvoiceID] = attempt
	}
	return attempt, nil
}

func (r *threadSafePaymentRepo) GetByID(
	ctx context.Context,
	db attemptrepo.DBTX,
	attemptID int64,
) (attemptmodel.PaymentAttempt, error) {
	return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPaymentAttemptNotFound
}

func (r *threadSafePaymentRepo) GetByInternalTxID(
	ctx context.Context,
	db attemptrepo.DBTX,
	internalTxID string,
) (attemptmodel.PaymentAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	att, exists := r.attemptsByTxID[internalTxID]
	if !exists {
		return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPaymentAttemptNotFound
	}
	return att, nil
}

func (r *threadSafePaymentRepo) GetByProviderTxID(
	ctx context.Context,
	db attemptrepo.DBTX,
	providerTxID string,
) (attemptmodel.PaymentAttempt, error) {
	return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPaymentAttemptNotFound
}

func (r *threadSafePaymentRepo) GetPendingByInvoiceID(
	ctx context.Context,
	db attemptrepo.DBTX,
	invoiceID int64,
) (attemptmodel.PaymentAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	att, exists := r.pendingByInvoice[invoiceID]
	if !exists {
		return attemptmodel.PaymentAttempt{}, attemptrepo.ErrPaymentAttemptNotFound
	}
	return att, nil
}

func (r *threadSafePaymentRepo) UpdateResult(
	ctx context.Context,
	db attemptrepo.DBTX,
	attemptID int64,
	status attemptmodel.Status,
	providerTxID *string,
	rawResponse json.RawMessage,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for k, att := range r.attemptsByTxID {
		if att.AttemptID == attemptID {
			att.Status = status
			att.ProviderTxID = providerTxID
			att.RawResponse = rawResponse
			r.attemptsByTxID[k] = att
			if status != attemptmodel.StatusPending {
				delete(r.pendingByInvoice, att.InvoiceID)
			}
			break
		}
	}
	return nil
}

func (r *threadSafePaymentRepo) ListByInvoiceID(
	ctx context.Context,
	db attemptrepo.DBTX,
	invoiceID int64,
) ([]attemptmodel.PaymentAttempt, error) {
	return nil, nil
}

func (r *threadSafePaymentRepo) DeleteByAttemptID(
	ctx context.Context,
	db attemptrepo.DBTX,
	attemptID int64,
) error {
	return nil
}

type mockInvoiceService struct{}

func (m *mockInvoiceService) Get(
	ctx context.Context,
	invoiceID int64,
) (invoicemodel.Invoice, error) {
	return invoicemodel.Invoice{
		InvoiceID: invoiceID,
		Status:    invoicemodel.StatusOpen,
		AmountDue: money.Money{AmountMinor: 1000, Currency: "USD"},
	}, nil
}

func (m *mockInvoiceService) CreateDraftInvoice(
	ctx context.Context,
	actor eventmodel.Actor,
	accountID int64,
	currency money.Currency,
	items []invoicemodel.LineItem,
) (invoicemodel.Invoice, error) {
	return invoicemodel.Invoice{}, nil
}

func (m *mockInvoiceService) FinalizeInvoice(
	ctx context.Context,
	actor eventmodel.Actor,
	invoiceID int64,
	paymentTermsDays int,
) (invoicemodel.Invoice, error) {
	return invoicemodel.Invoice{}, nil
}

func TestConcurrentWebhooksAndInvoicePayRequests(t *testing.T) {
	db := &threadSafeDB{}
	paymentRepo := newThreadSafePaymentRepo()
	attemptSvc := attemptservice.NewService(paymentRepo)

	ppiSvc := ppiservice.NewService(db, nil, attemptSvc, nil)
	ppiSvc.RegisterAdapter(fake.NewFakeAdapter())

	invSvc := &mockInvoiceService{}

	invoiceH := invoicehandler.NewInvoiceHandler(invSvc, ppiSvc)
	ppiH := ppihandler.NewPPIHandler(ppiSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/invoices/{invoiceID}/pay", invoiceH.HandlePayInvoice)
	mux.HandleFunc("POST /api/v1/webhooks/{provider}", ppiH.HandleWebhook)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	const concurrency = 20
	var wg sync.WaitGroup
	wg.Add(concurrency * 2)

	// Fire concurrent /invoices/{id}/pay requests
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			body, _ := json.Marshal(invoicehandler.PayInvoiceInput{
				ProviderCode:   "fake",
				IdempotencyKey: fmt.Sprintf("tx_worker_%d", workerID),
			})
			resp, err := ts.Client().Post(
				fmt.Sprintf("%s/api/v1/invoices/%d/pay", ts.URL, workerID+1),
				"application/json",
				bytes.NewReader(body),
			)
			if err != nil {
				t.Errorf("worker %d pay error: %v", workerID, err)
				return
			}
			_ = resp.Body.Close()
		}(i)
	}

	// Fire concurrent /webhooks/{provider} requests
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			payload := map[string]any{
				"event_id":  fmt.Sprintf("evt_concurrent_%d", workerID),
				"event":     "charge.success",
				"tx_ref":    fmt.Sprintf("tx_ref_%d", workerID),
				"reference": fmt.Sprintf("ref_%d", workerID),
				"status":    "success",
			}
			body, _ := json.Marshal(payload)
			resp, err := ts.Client().Post(
				fmt.Sprintf("%s/api/v1/webhooks/fake", ts.URL),
				"application/json",
				bytes.NewReader(body),
			)
			if err != nil {
				t.Errorf("worker %d webhook error: %v", workerID, err)
				return
			}
			_ = resp.Body.Close()
		}(i)
	}

	wg.Wait()
}
