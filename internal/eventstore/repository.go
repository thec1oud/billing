package eventstore

import (
	"context"

	"github.com/google/uuid"
	"github.com/thec1oud/billing/internal/shared/events"
)

type Repository interface {

	Append(
		ctx context.Context,
		event *events.Event,
	) error

	ReadStream(
		ctx context.Context,
		aggregateType events.AggregateType,
		aggregateID uuid.UUID,
	) ([]events.Event, error)
}