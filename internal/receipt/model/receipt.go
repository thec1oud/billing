package model

import (
	"fmt"
	"time"

	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
)

type Status string

const (
	StatusGenerated Status = "GENERATED"
	StatusFailed    Status = "FAILED"
	StatusDelivered Status = "DELIVERED"
)

// Receipt represents the metadata of a generated PDF receipt.
type Receipt struct {
	ReceiptID   string     `json:"receipt_id"`
	InvoiceID   int64      `json:"invoice_id"`
	AccountID   int64      `json:"account_id"`
	URL         string     `json:"url,omitempty"`
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

// ReceiptLineItem represents a single billed item on the receipt.
type ReceiptLineItem struct {
	Description   string  `json:"description"`
	QuantityValue float64 `json:"quantity_value"`
	QuantityUnit  string  `json:"quantity_unit"`
	UnitAmount    int64   `json:"unit_amount"`
	TotalAmount   int64   `json:"total_amount"`
}

// ReceiptPayload is the payload sent via Webhook and returned by the API.
// It contains both the raw data needed for custom rendering and a link to the PDF.
type ReceiptPayload struct {
	ReceiptID     string            `json:"receipt_id"`
	InvoiceID     int64             `json:"invoice_id"`
	InvoiceNumber string            `json:"invoice_number,omitempty"`
	AccountID     int64             `json:"account_id"`
	Currency      string            `json:"currency"`
	Subtotal      int64             `json:"subtotal"`
	Tax           int64             `json:"tax"`
	Total         int64             `json:"total"`
	AmountPaid    int64             `json:"amount_paid"`
	LineItems     []ReceiptLineItem `json:"line_items"`
	DownloadURL   string            `json:"download_url"`
	GeneratedAt   time.Time         `json:"generated_at"`
}

// GenerateReceiptID computes the deterministic receipt ID based on the invoice.
func GenerateReceiptID(invoiceID int64) string {
	return fmt.Sprintf("rcpt_%d", invoiceID)
}

// GenerateReceiptObjectKey computes the S3 object key based on account and receipt IDs.
func GenerateReceiptObjectKey(accountID int64, receiptID string) string {
	return fmt.Sprintf("receipts/%d/%s.pdf", accountID, receiptID)
}

// MapInvoiceToPayload maps an invoice and its metadata to the final API/Webhook payload.
func MapInvoiceToPayload(invoice *invoicemodel.Invoice, receiptID string, downloadURL string) ReceiptPayload {
	items := make([]ReceiptLineItem, len(invoice.LineItems))
	for i, li := range invoice.LineItems {
		items[i] = ReceiptLineItem{
			Description:   li.Description,
			QuantityValue: li.QuantityValue,
			QuantityUnit:  li.QuantityUnit,
			UnitAmount:    li.UnitAmount.AmountMinor,
			TotalAmount:   li.TotalAmount.AmountMinor,
		}
	}

	return ReceiptPayload{
		ReceiptID:     receiptID,
		InvoiceID:     invoice.InvoiceID,
		InvoiceNumber: invoice.InvoiceNumber,
		AccountID:     invoice.AccountID,
		Currency:      string(invoice.Currency),
		Subtotal:      invoice.Subtotal.AmountMinor,
		Tax:           invoice.Tax.AmountMinor,
		Total:         invoice.Total.AmountMinor,
		AmountPaid:    invoice.AmountPaid.AmountMinor,
		LineItems:     items,
		DownloadURL:   downloadURL,
		GeneratedAt:   time.Now().UTC(),
	}
}
