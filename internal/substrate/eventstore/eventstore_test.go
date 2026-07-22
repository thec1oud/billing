package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryEventStore struct {
	mu      sync.RWMutex
	streams map[string][]Event
}

func newMemoryEventStore() EventStore {
	return &memoryEventStore{
		streams: make(map[string][]Event),
	}
}

func (m *memoryEventStore) Append(_ context.Context, req AppendRequest) (uuid.UUID, error) {
	if req.Sequence < 1 {
		return uuid.Nil, ErrInvalidSequence
	}

	payloadBytes, err := json.Marshal(req.Payload)
	if err != nil {
		return uuid.Nil, err
	}

	eventID := uuid.New()
	key := fmt.Sprintf(
		"%s:%s",
		req.AggregateType,
		req.AggregateID.String(),
	)

	m.mu.Lock()
	defer m.mu.Unlock()

	stream := m.streams[key]

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
		OccurredAt:    time.Now().UTC(),
		CausationID:   req.CausationID,
		CorrelationID: req.CorrelationID,
	}

	m.streams[key] = append(stream, event)
	return eventID, nil
}

func (m *memoryEventStore) ReadStream(
	_ context.Context,
	aggregateType AggregateType,
	aggregateID uuid.UUID,
) ([]Event, error) {
	key := fmt.Sprintf("%s:%s", aggregateType, aggregateID.String())

	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[key]
	if !exists {
		return []Event{}, nil
	}

	result := make([]Event, len(stream))
	for i, event := range stream {
		result[i] = event
		result[i].Payload = append(json.RawMessage(nil), event.Payload...)
	}

	return result, nil
}

func TestA1_Append5EventsAndVerifyOrder(t *testing.T) {
	ctx := context.Background()
	store := newMemoryEventStore()
	targetID := uuid.New()
	targetType := AggregateAccount

	for i := int64(1); i <= 5; i++ {
		_, err := store.Append(ctx, AppendRequest{
			AggregateType: targetType,
			AggregateID:   targetID,
			Sequence:      i,
			EventType:     AccountCreated,
			EventVersion:  1,
			Actor:         "test-suite",
			Payload:       map[string]int{"event_index": int(i)},
		})
		if err != nil {
			t.Fatalf("Failed to append event [%d]: %v", i, err)
		}
	}

	stream, err := store.ReadStream(ctx, targetType, targetID)
	if err != nil {
		t.Fatalf("Failed to read stream: %v", err)
	}

	if len(stream) != 5 {
		t.Fatalf("Expected 5 events, got %d", len(stream))
	}

	for index, event := range stream {
		expectedSequence := int64(index + 1)
		if event.Sequence != expectedSequence {
			t.Errorf("Sequence mismatch: expected %d, got %d", expectedSequence, event.Sequence)
		}

		expectedJSON := fmt.Sprintf(`{"event_index":%d}`, expectedSequence)
		if string(event.Payload) != expectedJSON {
			t.Errorf("Payload mismatch: expected %s, got %s", expectedJSON, string(event.Payload))
		}
	}
}

func TestA1_RejectStaleOrDuplicateSequence(t *testing.T) {
	ctx := context.Background()
	store := newMemoryEventStore()
	targetID := uuid.New()
	targetType := AggregateSubscription

	_, err := store.Append(ctx, AppendRequest{
		AggregateType: targetType,
		AggregateID:   targetID,
		Sequence:      1,
		EventType:     SubscriptionCreated,
		Payload:       "initial_state",
	})
	if err != nil {
		t.Fatalf("Initial append failed: %v", err)
	}

	_, errDuplicate := store.Append(ctx, AppendRequest{
		AggregateType: targetType,
		AggregateID:   targetID,
		Sequence:      1,
		EventType:     SubscriptionCreated,
		Payload:       "duplicate_state",
	})
	if errDuplicate != ErrSequenceConflict {
		t.Errorf("Expected ErrSequenceConflict for duplicate, got: %v", errDuplicate)
	}

	_, errGap := store.Append(ctx, AppendRequest{
		AggregateType: targetType,
		AggregateID:   targetID,
		Sequence:      3,
		EventType:     SubscriptionCreated,
		Payload:       "gap_state",
	})
	if errGap != ErrSequenceConflict {
		t.Errorf("Expected ErrSequenceConflict for sequence gap, got: %v", errGap)
	}
}

func TestA2_ProveGenericRebuildProjection(t *testing.T) {
	historicalStream := []Event{
		{Sequence: 1, EventType: "INC"},
		{Sequence: 2, EventType: "INC"},
		{Sequence: 3, EventType: "DEC"},
		{Sequence: 4, EventType: "INC"},
	}

	countReducer := func(currentVal int, ev Event) (int, error) {
		switch string(ev.EventType) {
		case "INC":
			return currentVal + 1, nil
		case "DEC":
			return currentVal - 1, nil
		default:
			return currentVal, nil
		}
	}

	finalState, err := Rebuild(0, historicalStream, countReducer)
	if err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	if finalState != 2 {
		t.Errorf("Expected 2, got %d", finalState)
	}
}
