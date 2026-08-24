package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/ppi"
)

var log = logger.ForComponent("ppi_subscriber")

type WebhookSubscriberFunc func(ctx context.Context, payload ppi.ProviderWebhookPayload) error

func RegisterModuleSubscriber(
	ctx context.Context,
	broker messaging.Broker,
	queueName string,
	routingKeys []string,
	handler WebhookSubscriberFunc,
) error {
	if broker == nil {
		return fmt.Errorf("broker is nil")
	}

	consumerFunc := func(ctx context.Context, msg []byte) error {
		var payload ppi.ProviderWebhookPayload
		if err := json.Unmarshal(msg, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal ProviderWebhookPayload from message: %w", err)
		}

		log.Info("Subscriber received webhook message from queue",
			slog.String("queue", queueName),
			slog.String("webhook_id", payload.WebhookID),
			slog.String("provider_code", payload.ProviderCode),
			slog.String("internal_tx_id", payload.InternalTxID),
			slog.String("status", string(payload.Status)),
		)

		return handler(ctx, payload)
	}

	return broker.RegisterConsumerGroup(ctx, queueName, routingKeys, consumerFunc)
}
