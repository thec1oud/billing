package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/infra/storage"
	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/receipt/model"
	"github.com/thec1oud/billing/internal/receipt/service"
)

var log = logger.ForComponent("receipt_worker")

// InvoiceFetcher defines the interface for fetching invoices, decoupling the receipt module from the invoice repository.
type InvoiceFetcher interface {
	Get(ctx context.Context, invoiceID int64) (invoicemodel.Invoice, error)
}

type Consumer struct {
	broker        messaging.Broker
	invoiceFetcher InvoiceFetcher
	generator     service.GeneratorService
	delivery      service.DeliveryService
	storage       storage.ObjectStorage
}

func NewConsumer(
	broker messaging.Broker,
	invoiceFetcher InvoiceFetcher,
	generator service.GeneratorService,
	delivery service.DeliveryService,
	storage storage.ObjectStorage,
) *Consumer {
	return &Consumer{
		broker:         broker,
		invoiceFetcher: invoiceFetcher,
		generator:      generator,
		delivery:       delivery,
		storage:        storage,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	// The routing key for paid invoices.
	// We assume it's published to the domain exchange with "invoice.paid"
	routingKeys := []string{"invoice.paid"}
	queueName := "billing.receipt.generator"

	err := c.broker.RegisterConsumerGroup(ctx, queueName, routingKeys, c.handleInvoicePaid)
	if err != nil {
		return fmt.Errorf("failed to register receipt consumer: %w", err)
	}

	return nil
}

func (c *Consumer) handleInvoicePaid(ctx context.Context, msg []byte) error {
	// 1. Parse the event payload
	// The event payload for "invoice.paid" should contain at least the InvoiceID.
	// We use an anonymous struct if it differs, or use a map
	var payload map[string]interface{}
	if err := json.Unmarshal(msg, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal invoice.paid event: %w", err)
	}

	invoiceIDFloat, ok := payload["invoice_id"].(float64)
	if !ok {
		return fmt.Errorf("invoice_id missing or invalid in payload")
	}
	invoiceID := int64(invoiceIDFloat)

	log.Info("Receipt worker received invoice.paid event", slog.Int64("invoice_id", invoiceID))

	// 2. Fetch the invoice details
	invoice, err := c.invoiceFetcher.Get(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("failed to fetch invoice %d: %w", invoiceID, err)
	}

	// 3. Generate the PDF
	pdfBytes, err := c.generator.Generate(&invoice)
	if err != nil {
		return fmt.Errorf("failed to generate PDF for invoice %d: %w", invoiceID, err)
	}

	// 4. Upload to Object Storage
	receiptID := uuid.New().String()
	objectKey := fmt.Sprintf("receipts/%d/%s.pdf", invoice.AccountID, receiptID)

	err = c.storage.UploadFile(ctx, objectKey, bytes.NewReader(pdfBytes), "application/pdf")
	if err != nil {
		return fmt.Errorf("failed to upload receipt to storage: %w", err)
	}

	// 5. Generate Pre-signed URL (Valid for 7 days)
	downloadURL, err := c.storage.GetPresignedURL(ctx, objectKey, 7*24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	// 6. Deliver via Webhook
	receiptPayload := model.ReceiptPayload{
		ReceiptID:     receiptID,
		InvoiceID:     invoice.InvoiceID,
		InvoiceNumber: invoice.InvoiceNumber,
		AccountID:     invoice.AccountID,
		DownloadURL:   downloadURL,
		GeneratedAt:   time.Now().UTC(),
	}

	err = c.delivery.Deliver(ctx, receiptPayload)
	if err != nil {
		log.Error("Failed to deliver receipt webhook", slog.String("receipt_id", receiptID), logger.Err(err))
		return err // Returning an error nacks the message, sending it to DLQ for retry/inspection
	}

	log.Info("Successfully generated and delivered receipt", slog.String("receipt_id", receiptID), slog.Int64("invoice_id", invoiceID))
	return nil
}
