package events

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence boundary for the append-only event store.
type Repository interface {
	Append(ctx context.Context, event *Event) error
	ReadStream(ctx context.Context, aggregateType AggregateType, aggregateID uuid.UUID) ([]Event, error)
}
