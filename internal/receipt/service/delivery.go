package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/receipt/model"
)


type DeliveryService interface {
	Deliver(ctx context.Context, payload model.ReceiptPayload) error
}

type webhookDeliveryService struct {
	cfg        *config.Config
	httpClient *http.Client
	log        *slog.Logger
}

func NewDeliveryService(cfg *config.Config) DeliveryService {
	return &webhookDeliveryService{
		cfg:        cfg,
		httpClient: &http.Client{},
		log:        logger.ForComponent("receipt_delivery"),
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

	maxRetries := 3
	baseDelay := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.ReceiptWebhookURL, bytes.NewBuffer(body))
		if err != nil {
			return fmt.Errorf("failed to create webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.httpClient.Do(req)

		if err == nil {
			resp.Body.Close() // Safe to close here inside loop
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil // Success!
			}

			// Do not retry on 4xx client errors, except 429 (Too Many Requests)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
				return fmt.Errorf("webhook delivery failed with non-retryable status code: %d", resp.StatusCode)
			}
		}

		if attempt == maxRetries {
			if err != nil {
				return fmt.Errorf("failed to send webhook after %d attempts: %w", maxRetries, err)
			}
			return fmt.Errorf("webhook delivery failed after %d attempts with status code: %d", maxRetries, resp.StatusCode)
		}

		delay := baseDelay * time.Duration(1<<attempt) // 1s, 2s, 4s...

		if err != nil {
			s.log.Warn("Webhook request failed, retrying", slog.Int("attempt", attempt+1), slog.Duration("delay", delay), logger.Err(err))
		} else {
			s.log.Warn(
				"Webhook returned failure status, retrying",
				slog.Int("attempt", attempt+1),
				slog.Duration("delay", delay),
				slog.Int("status", resp.StatusCode),
			)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// continue to next attempt
		}
	}

	return nil
}
