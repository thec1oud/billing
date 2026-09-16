package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/receipt/model"
)

type DeliveryService interface {
	Deliver(ctx context.Context, payload model.ReceiptPayload) error
}

type webhookDeliveryService struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewDeliveryService(cfg *config.Config) DeliveryService {
	return &webhookDeliveryService{
		cfg: cfg,
		httpClient: &http.Client{},
	}
}

func (s *webhookDeliveryService) Deliver(ctx context.Context, payload model.ReceiptPayload) error {
	if s.cfg.ReceiptWebhookURL == "" {
		// If no URL is configured, we can either return an error or just log it.
		// For a standalone client, this should be mandatory.
		return fmt.Errorf("receipt webhook URL is not configured")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal receipt payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.ReceiptWebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook delivery failed with status code: %d", resp.StatusCode)
	}

	return nil
}
