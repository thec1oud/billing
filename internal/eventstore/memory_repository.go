package eventstore

import (
	"context"
	"sync"

	"github.com/google/uuid"

	sharedEvents "github.com/thec1oud/billing/internal/shared/events"
)

// MemoryRepository is a concurrency-safe event-store implementation intended for
// local development and unit tests. It preserves the same sequence contract as
// the PostgreSQL repository.
type MemoryRepository struct {
	mu      sync.RWMutex
	streams map[streamKey][]sharedEvents.Event
}

type streamKey struct {
	aggregateType sharedEvents.AggregateType
	aggregateID   uuid.UUID
}

func NewMemoryRepository() Repository {
	return &MemoryRepository{streams: make(map[streamKey][]sharedEvents.Event)}
}

func (r *MemoryRepository) Append(_ context.Context, event *sharedEvents.Event) error {
	if event.Sequence < 1 {
		return ErrInvalidSequence
	}

	key := streamKey{aggregateType: event.AggregateType, aggregateID: event.AggregateID}

	r.mu.Lock()
	defer r.mu.Unlock()

	stream := r.streams[key]
	expectedSequence := int64(len(stream) + 1)
	if event.Sequence != expectedSequence {
		return ErrSequenceConflict
	}

	// Copy the values owned by the store so a caller cannot mutate its history.
	stored := *event
	stored.Payload = append([]byte(nil), event.Payload...)
	r.streams[key] = append(stream, stored)
	return nil
}

func (r *MemoryRepository) ReadStream(_ context.Context, aggregateType sharedEvents.AggregateType, aggregateID uuid.UUID) ([]sharedEvents.Event, error) {
	key := streamKey{aggregateType: aggregateType, aggregateID: aggregateID}

	r.mu.RLock()
	defer r.mu.RUnlock()

	stream := r.streams[key]
	result := make([]sharedEvents.Event, len(stream))
	for i, event := range stream {
		result[i] = event
		result[i].Payload = append([]byte(nil), event.Payload...)
	}
	return result, nil
}
