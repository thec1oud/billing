package invoice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/ppi"
	sharedUUID "github.com/thec1oud/billing/internal/shared/uuid"
	"github.com/thec1oud/billing/internal/substrate/events"
	"github.com/thec1oud/billing/internal/substrate/idempotency"
	"github.com/thec1oud/billing/internal/substrate/money"
)

var (
	ErrInvoiceNotFound   = errors.New("invoice not found")
	ErrInvoiceNotMutable = errors.New("invoice is not mutable outside DRAFT status")
)

type PlanLookup interface {
	FlatFeeForSubscription(ctx context.Context, subscriptionID uuid.UUID) (accountID uuid.UUID, fee money.Money, periodStart, periodEnd time.Time, err error)
}

type Service struct {
	store    *events.EventStore
	idem     *idempotency.Store
	plans    PlanLookup
	accounts AccountLookup
	ppi      PPI
}

func NewService(store *events.EventStore, idem *idempotency.Store, plans PlanLookup, accounts AccountLookup, ppi PPI) *Service {
	return &Service{store: store, idem: idem, plans: plans, accounts: accounts, ppi: ppi}
}

func (s *Service) currentState(ctx context.Context, invoiceID uuid.UUID) (Invoice, []events.Event, error) {
	stream, err := s.store.ReadStream(ctx, events.AggregateInvoice, invoiceID)
	if err != nil {
		return Invoice{}, nil, fmt.Errorf("read invoice stream: %w", err)
	}
	if len(stream) == 0 {
		return Invoice{}, nil, ErrInvoiceNotFound
	}
	state, err := events.Rebuild(Invoice{}, stream, Reduce)
	if err != nil {
		return Invoice{}, nil, fmt.Errorf("rebuild invoice state: %w", err)
	}
	return state, stream, nil
}

func (s *Service) CreateDraftInvoice(ctx context.Context, subscriptionID uuid.UUID, idempotencyKey string) (Invoice, error) {
	requestHash := idempotency.HashRequest([]byte(subscriptionID.String()))

	decision, err := s.idem.CheckOrReserve(ctx, idempotencyKey, "invoice.create_draft", requestHash)
	if err != nil {
		return Invoice{}, fmt.Errorf("idempotency check: %w", err)
	}
	if !decision.ShouldProceed() {
		var cached Invoice
		if err := json.Unmarshal(decision.CachedResponse, &cached); err != nil {
			return Invoice{}, fmt.Errorf("unmarshal cached invoice: %w", err)
		}
		return cached, nil
	}

	accountID, fee, periodStart, periodEnd, err := s.plans.FlatFeeForSubscription(ctx, subscriptionID)
	if err != nil {
		return Invoice{}, fmt.Errorf("look up plan for subscription %s: %w", subscriptionID, err)
	}

	invoiceID, err := sharedUUID.New()
	if err != nil {
		return Invoice{}, fmt.Errorf("generate invoice id: %w", err)
	}

	lineItems := []LineItem{
		{Description: "Subscription fee", Amount: fee, PeriodStart: periodStart, PeriodEnd: periodEnd},
	}

	payload := CreatedPayload{
		AccountID:      accountID,
		SubscriptionID: subscriptionID,
		Currency:       fee.Currency,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		LineItems:      lineItems,
		Total:          fee,
	}

	if _, err := s.store.Append(ctx, events.AppendRequest{
		AggregateType: events.AggregateInvoice,
		AggregateID:   invoiceID,
		Sequence:      1, // new aggregate — always the first event in its stream
		EventType:     events.InvoiceCreated,
		EventVersion:  1,
		Actor:         "system",
		Payload:       payload,
	}); err != nil {
		return Invoice{}, fmt.Errorf("append InvoiceCreated: %w", err)
	}

	inv := Invoice{
		InvoiceID:      invoiceID,
		AccountID:      accountID,
		SubscriptionID: subscriptionID,
		Status:         StatusDraft,
		Currency:       fee.Currency,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		LineItems:      lineItems,
		Total:          fee,
	}

	if err := s.idem.StoreResponse(ctx, *decision.ProceedToken, inv); err != nil {
		return Invoice{}, fmt.Errorf("store idempotent response: %w", err)
	}

	return inv, nil
}

func (s *Service) FinalizeInvoice(ctx context.Context, invoiceID uuid.UUID) (Invoice, error) {
	inv, stream, err := s.currentState(ctx, invoiceID)
	if err != nil {
		return Invoice{}, err
	}
	if inv.Status != StatusDraft {
		return Invoice{}, ErrInvoiceNotMutable
	}

	nextSequence := int64(len(stream)) + 1

	if _, err := s.store.Append(ctx, events.AppendRequest{
		AggregateType: events.AggregateInvoice,
		AggregateID:   invoiceID,
		Sequence:      nextSequence,
		EventType:     events.InvoiceFinalized,
		EventVersion:  1,
		Actor:         "system",
		Payload:       FinalizedPayload{},
	}); err != nil {
		if errors.Is(err, events.ErrSequenceConflict) {
			return Invoice{}, fmt.Errorf("invoice was modified concurrently, retry: %w", err)
		}
		return Invoice{}, fmt.Errorf("append InvoiceFinalized: %w", err)
	}

	inv.Status = StatusOpen
	return inv, nil
}

type PPI interface {
	ChargePaymentMethod(ctx context.Context, amount money.Money, paymentMethodID, idempotencyKey string) (ppi.ChargeResult, error)
}

func (s *Service) AttemptPayment(ctx context.Context, invoiceID uuid.UUID, idempotencyKey string) (Invoice, error) {
	requestHash := idempotency.HashRequest([]byte(invoiceID.String()))

	decision, err := s.idem.CheckOrReserve(ctx, idempotencyKey, "invoice.attempt_payment", requestHash)
	if err != nil {
		return Invoice{}, fmt.Errorf("idempotency check: %w", err)
	}
	if !decision.ShouldProceed() {
		var cached Invoice
		if err := json.Unmarshal(decision.CachedResponse, &cached); err != nil {
			return Invoice{}, fmt.Errorf("unmarshal cached invoice: %w", err)
		}
		return cached, nil
	}

	inv, stream, err := s.currentState(ctx, invoiceID)
	if err != nil {
		return Invoice{}, err
	}
	if inv.Status != StatusOpen {
		return Invoice{}, fmt.Errorf("cannot attempt payment on invoice in status %s: %w", inv.Status, ErrInvoiceNotMutable)
	}

	paymentMethodID, err := s.accounts.DefaultPaymentMethodID(ctx, inv.AccountID)
	if err != nil {
		return Invoice{}, fmt.Errorf("look up payment method for account %s: %w", inv.AccountID, err)
	}

	seq := int64(len(stream)) + 1

	if _, err := s.store.Append(ctx, events.AppendRequest{
		AggregateType: events.AggregateInvoice,
		AggregateID:   invoiceID,
		Sequence:      seq,
		EventType:     events.PaymentAttempted,
		EventVersion:  1,
		Actor:         "system",
		Payload: PaymentAttemptedPayload{
			PaymentMethodID: paymentMethodID,
			Amount:          inv.Total,
		},
	}); err != nil {
		return Invoice{}, fmt.Errorf("append PaymentAttempted: %w", err)
	}
	seq++

	// The PPI call uses a key derived from idempotencyKey, not the same key
	// directly — idempotency.Store namespaces by key alone, not by
	// (key, operation_type), so reusing the raw key across attempt_payment's
	// own reservation and the PPI charge's reservation causes a spurious
	// IDEMPOTENCY_CONFLICT. Deriving a distinct key preserves the retry
	// guarantee (same input key → same derived key → same cached charge)
	// while keeping the two reservations independent.
	chargeIdempotencyKey := idempotencyKey + ":charge"
	chargeResult, err := s.ppi.ChargePaymentMethod(ctx, inv.Total, paymentMethodID, chargeIdempotencyKey)
	if err != nil {
		return Invoice{}, fmt.Errorf("charge payment method: %w", err)
	}

	switch chargeResult.Status {
	case ppi.ChargeStatusSuccess:
		if _, err := s.store.Append(ctx, events.AppendRequest{
			AggregateType: events.AggregateInvoice,
			AggregateID:   invoiceID,
			Sequence:      seq,
			EventType:     events.PaymentSucceeded,
			EventVersion:  1,
			Actor:         "system",
			Payload:       PaymentSucceededPayload{ProviderReference: chargeResult.ProviderReference},
		}); err != nil {
			return Invoice{}, fmt.Errorf("append PaymentSucceeded: %w", err)
		}
		seq++

		if _, err := s.store.Append(ctx, events.AppendRequest{
			AggregateType: events.AggregateInvoice,
			AggregateID:   invoiceID,
			Sequence:      seq,
			EventType:     events.InvoicePaid,
			EventVersion:  1,
			Actor:         "system",
			Payload:       InvoicePaidPayload{},
		}); err != nil {
			return Invoice{}, fmt.Errorf("append InvoicePaid: %w", err)
		}
		inv.Status = StatusPaid

	default: // FAILED and anything else the fake adapter doesn't produce yet
		if _, err := s.store.Append(ctx, events.AppendRequest{
			AggregateType: events.AggregateInvoice,
			AggregateID:   invoiceID,
			Sequence:      seq,
			EventType:     events.PaymentFailed,
			EventVersion:  1,
			Actor:         "system",
			Payload:       PaymentFailedPayload{FailureCode: chargeResult.FailureCode},
		}); err != nil {
			return Invoice{}, fmt.Errorf("append PaymentFailed: %w", err)
		}
		// invoice stays OPEN — no retry/dunning logic yet, per C3 spec
	}

	if err := s.idem.StoreResponse(ctx, *decision.ProceedToken, inv); err != nil {
		return Invoice{}, fmt.Errorf("store idempotent response: %w", err)
	}

	return inv, nil
}
