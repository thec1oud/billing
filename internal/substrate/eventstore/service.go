package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	sharedUUID "github.com/thec1oud/billing/internal/shared/uuid"
)

// EventStore provides the application-facing append and read operations.
type EventStore struct {
	repository Repository
}

func NewEventStore(repository Repository) *EventStore {
	return &EventStore{repository: repository}
}

func (s *EventStore) Append(ctx context.Context, req AppendRequest) (uuid.UUID, error) {
	eventID, err := sharedUUID.New()
	if err != nil {
		return uuid.Nil, err
	}
	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return uuid.Nil, err
	}

	event := &Event{
		EventID:       eventID,
		EventType:     req.EventType,
		EventVersion:  req.EventVersion,
		AggregateType: req.AggregateType,
		AggregateID:   req.AggregateID,
		Sequence:      req.Sequence,
		Actor:         req.Actor,
		Payload:       payload,
		OccurredAt:    time.Now().UTC().Truncate(time.Microsecond),
		CausationID:   req.CausationID,
		CorrelationID: req.CorrelationID,
	}
	if err := s.repository.Append(ctx, event); err != nil {
		return uuid.Nil, err
	}
	return eventID, nil
}

func (s *EventStore) ReadStream(ctx context.Context, aggregateType AggregateType, aggregateID uuid.UUID) ([]Event, error) {
	return s.repository.ReadStream(ctx, aggregateType, aggregateID)
}
