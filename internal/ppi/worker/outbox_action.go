package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
)

type PublishWebhookPayload struct {
	RoutingKey string          `json:"routing_key"`
	Payload    json.RawMessage `json:"payload"`
}

type PublishWebhookAction struct {
	broker messaging.Broker
}

func NewPublishWebhookAction(broker messaging.Broker) *PublishWebhookAction {
	return &PublishWebhookAction{broker: broker}
}

func (a *PublishWebhookAction) Name() string {
	return "PublishWebhook"
}

func (a *PublishWebhookAction) Execute(ctx context.Context, db sqlcgen.DBTX, ec *model.ExecutionContext, params json.RawMessage) error {
	var p PublishWebhookPayload
	if err := json.Unmarshal(params, &p); err != nil {
		return fmt.Errorf("failed to unmarshal publish webhook params: %w", err)
	}

	if err := a.broker.PublishEvent(ctx, p.RoutingKey, p.Payload); err != nil {
		return fmt.Errorf("failed to publish webhook to broker: %w", err)
	}

	return nil
}
