package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/receipt/model"
)

func TestDeliveryService_Deliver_Success(t *testing.T) {
	// 1. Create a local httptest server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Verify body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload model.ReceiptPayload
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.InvoiceID != 123 {
			t.Errorf("expected InvoiceID 123, got %d", payload.InvoiceID)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 2. Configure the service to use our test server URL
	cfg := &config.Config{
		ReceiptWebhookURL: server.URL,
	}
	svc := NewDeliveryService(cfg)

	// 3. Execute the delivery
	payload := model.ReceiptPayload{
		InvoiceID:  123,
		AccountID:  456,
		DownloadURL: "https://example.com/receipt.pdf",
		GeneratedAt: time.Now(),
	}

	err := svc.Deliver(context.Background(), payload)
	if err != nil {
		t.Errorf("expected no error on delivery, got: %v", err)
	}
}

func TestDeliveryService_Deliver_NoURLConfigured(t *testing.T) {
	cfg := &config.Config{
		ReceiptWebhookURL: "",
	}
	svc := NewDeliveryService(cfg)

	err := svc.Deliver(context.Background(), model.ReceiptPayload{})
	if err == nil {
		t.Error("expected error due to missing webhook URL, got nil")
	}
}

func TestDeliveryService_Deliver_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		ReceiptWebhookURL: server.URL,
	}
	svc := NewDeliveryService(cfg)

	err := svc.Deliver(context.Background(), model.ReceiptPayload{InvoiceID: 123})
	if err == nil {
		t.Error("expected error due to 500 server response, got nil")
	}
}
