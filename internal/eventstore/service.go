package eventstore

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	sharedEvents "github.com/thec1oud/billing/internal/shared/events"
	sharedUUID "github.com/thec1oud/billing/internal/shared/uuid"
)

type EventStore struct {
	repository Repository
}

func New(repository Repository) *EventStore {
	return &EventStore{
		repository: repository,
	}
}

func (s *EventStore) Append(
	ctx context.Context,
	req AppendRequest,
) (uuid.UUID, error) {

	eventID, err := sharedUUID.New()
	if err != nil {
		return uuid.Nil, err
	}

	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return uuid.Nil, err
	}

	event := &sharedEvents.Event{
		EventID:         eventID,
		EventType:       req.EventType,
		EventVersion:    req.EventVersion,
		AggregateType:   req.AggregateType,
		AggregateID:     req.AggregateID,
		Sequence:        req.Sequence,
		Actor:           req.Actor,
		Payload:         payload,
		OccurredAt:      time.Now().UTC().Truncate(time.Microsecond),
		CausationID:     req.CausationID,
		CorrelationID:   req.CorrelationID,
	}

	if err := s.repository.Append(ctx, event); err != nil {
		return uuid.Nil, err
	}

	return eventID, nil
}

func (s *EventStore) ReadStream(
	ctx context.Context,
	aggregateType sharedEvents.AggregateType,
	aggregateID uuid.UUID,
) ([]sharedEvents.Event, error) {

	return s.repository.ReadStream(
		ctx,
		aggregateType,
		aggregateID,
	)
}