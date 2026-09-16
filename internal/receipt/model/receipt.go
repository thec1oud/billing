package model

import (
	"time"
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

// ReceiptPayload is the payload sent via Webhook to the requesting server.
type ReceiptPayload struct {
	ReceiptID     string    `json:"receipt_id"`
	InvoiceID     int64     `json:"invoice_id"`
	InvoiceNumber string    `json:"invoice_number,omitempty"`
	AccountID     int64     `json:"account_id"`
	DownloadURL   string    `json:"download_url"`
	GeneratedAt   time.Time `json:"generated_at"`
}
