package events

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	streams map[streamKey][]Event
}

type streamKey struct {
	aggregateType AggregateType
	aggregateID   uuid.UUID
}

func NewMemoryRepository() Repository {
	return &MemoryRepository{streams: make(map[streamKey][]Event)}
}

func (r *MemoryRepository) Append(_ context.Context, event *Event) error {
	if event.Sequence < 1 {
		return ErrInvalidSequence
	}

	key := streamKey{aggregateType: event.AggregateType, aggregateID: event.AggregateID}
	r.mu.Lock()
	defer r.mu.Unlock()

	stream := r.streams[key]
	if event.Sequence != int64(len(stream)+1) {
		return ErrSequenceConflict
	}

	stored := *event
	stored.Payload = append([]byte(nil), event.Payload...)
	r.streams[key] = append(stream, stored)
	return nil
}

func (r *MemoryRepository) ReadStream(
	_ context.Context,
	aggregateType AggregateType,
	aggregateID uuid.UUID,
) ([]Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stream := r.streams[streamKey{aggregateType: aggregateType, aggregateID: aggregateID}]
	result := make([]Event, len(stream))
	for i, event := range stream {
		result[i] = event
		result[i].Payload = append([]byte(nil), event.Payload...)
	}
	return result, nil
}
