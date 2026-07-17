package events

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	sharedUUID "github.com/thec1oud/billing/internal/shared/uuid"
)

type MemoryEventStore struct {
	mu      sync.RWMutex
	streams map[streamKey][]Event
}

type streamKey struct {
	aggregateType AggregateType
	aggregateID   uuid.UUID
}

func NewMemoryEventStore() EventStore {
	return &MemoryEventStore{
		streams: make(map[streamKey][]Event),
	}
}

func (m *MemoryEventStore) Append(_ context.Context, req AppendRequest) (uuid.UUID, error) {
	if req.Sequence < 1 {
		return uuid.Nil, ErrInvalidSequence
	}

	//Marshall Payload and check IDs natively inside the store execution boundary
	payloadBytes, err := json.Marshal(req.Payload)
	if err != nil {
		return uuid.Nil, err
	}

	eventID, err := sharedUUID.New()
	if err != nil {
		return uuid.Nil, err
	}

	key := streamKey{aggregateType: req.AggregateType, aggregateID: req.AggregateID}

	m.mu.Lock()
	defer m.mu.Unlock()

	stream := m.streams[key]
	
	// Monotonically increasing sequencing integrity assertion check
	if req.Sequence != int64(len(stream)+1) {
		return uuid.Nil, ErrSequenceConflict
	}

	event := Event{
		EventID:       eventID,
		EventType:     req.EventType,
		EventVersion:  req.EventVersion,
		AggregateType: req.AggregateType,
		AggregateID:   req.AggregateID,
		Sequence:      req.Sequence,
		Actor:         req.Actor,
		Payload:       payloadBytes,
		OccurredAt:    time.Now().UTC().Truncate(time.Microsecond),
		CausationID:   req.CausationID,
		CorrelationID: req.CorrelationID,
	}

	m.streams[key] = append(stream, event)
	return eventID, nil
}

func (m *MemoryEventStore) ReadStream(
	_ context.Context,
	aggregateType AggregateType,
	aggregateID uuid.UUID,
) ([]Event, error) {
	key := streamKey{aggregateType: aggregateType, aggregateID: aggregateID}

	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[key]
	if !exists {
		return []Event{}, nil
	}

	// Return deep data copies ensuring memory allocation encapsulation isolation arrays
	result := make([]Event, len(stream))
	for i, event := range stream {
		result[i] = event
		result[i].Payload = append(json.RawMessage(nil), event.Payload...)
	}

	return result, nil
}
