package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/thec1oud/billing/internal/infra/storage"
	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/receipt/model"
)

// InvoiceFetcher defines the interface for fetching invoices for the receipt handler.
type InvoiceFetcher interface {
	Get(ctx context.Context, invoiceID int64) (invoicemodel.Invoice, error)
}

type ReceiptHandler struct {
	invoiceFetcher InvoiceFetcher
	storage        storage.ObjectStorage
}

func NewReceiptHandler(invoiceFetcher InvoiceFetcher, storage storage.ObjectStorage) *ReceiptHandler {
	return &ReceiptHandler{
		invoiceFetcher: invoiceFetcher,
		storage:        storage,
	}
}

// GetReceiptData handles requests to fetch the raw receipt data (payload and presigned URL) for a given invoice.
func (h *ReceiptHandler) GetReceiptData(w http.ResponseWriter, r *http.Request) {
	invoiceIDStr := r.PathValue("invoice_id")
	if invoiceIDStr == "" {
		http.Error(w, "missing invoice_id parameter", http.StatusBadRequest)
		return
	}

	invoiceID, err := strconv.ParseInt(invoiceIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid invoice_id parameter", http.StatusBadRequest)
		return
	}

	// 1. Fetch the associated invoice
	invoice, err := h.invoiceFetcher.Get(r.Context(), invoiceID)
	if err != nil {
		http.Error(w, "failed to fetch invoice", http.StatusNotFound)
		return
	}

	// 2. Reconstruct the deterministic Receipt ID and S3 Object Key
	receiptID := model.GenerateReceiptID(invoice.InvoiceID)
	objectKey := model.GenerateReceiptObjectKey(invoice.AccountID, receiptID)

	// 3. Generate a fresh presigned URL valid for 7 days
	downloadURL, err := h.storage.GetPresignedURL(r.Context(), objectKey, 7*24*time.Hour)
	if err != nil {
		http.Error(w, "failed to generate download URL", http.StatusInternalServerError)
		return
	}

	// 4. Construct the raw data payload
	payload := model.MapInvoiceToPayload(&invoice, receiptID, downloadURL)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
