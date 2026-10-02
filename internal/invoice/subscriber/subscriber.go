package subscriber

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/infra/logger"
	invoicesvc "github.com/thec1oud/billing/internal/invoice/service"
	attemptsvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
)

var log = logger.ForComponent("invoice_subscriber")

type InvoiceSubscriber struct {
	db         *pgxpool.Pool
	invoiceSvc *invoicesvc.Service
	attemptSvc *attemptsvc.Service
}

func NewInvoiceSubscriber(
	db *pgxpool.Pool,
	invoiceSvc *invoicesvc.Service,
	attemptSvc *attemptsvc.Service,
) *InvoiceSubscriber {
	return &InvoiceSubscriber{
		db:         db,
		invoiceSvc: invoiceSvc,
		attemptSvc: attemptSvc,
	}
}

// HandlePaymentWebhook is designed to be registered via ppi/subscriber.RegisterModuleSubscriber
func (s *InvoiceSubscriber) HandlePaymentWebhook(ctx context.Context, payload ppi.ProviderWebhookPayload) error {
	// We only care about successful payments to transition the invoice to paid.
	if payload.Status != ppi.WebhookPaymentSucceeded {
		return nil
	}

	// 1. Find the PaymentAttempt to get the associated InvoiceID
	attempt, err := s.attemptSvc.GetAttemptByInternalTxID(ctx, s.db, payload.InternalTxID)
	if err != nil {
		log.Error("Failed to fetch payment attempt for webhook", logger.Err(err))
		return fmt.Errorf("fetch payment attempt: %w", err)
	}

	// 2. Fire the state machine transition
	// We use the actor "system" since this is triggered by an automated webhook.
	actor := eventmodel.Actor{Type: "system", ID: "ppi_webhook"}

	// Assuming the full amount due is paid by this attempt.
	// For partial payments, we'd need to fetch the invoice and subtract.
	err = s.invoiceSvc.PayInvoice(
		ctx,
		actor,
		attempt.InvoiceID,
		payload.Amount,
		payload.Amount,
		payload.OccurredAt,
	)
	if err != nil {
		log.Error("Failed to trigger PayInvoice transition", logger.Err(err))
		return fmt.Errorf("trigger invoice payment: %w", err)
	}

	log.Info("Successfully triggered invoice payment via webhook", "invoice_id", attempt.InvoiceID)
	return nil
}
