package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	events "github.com/thec1oud/billing/internal/substrate/eventstore"
)

type testPayload struct {
	Number int `json:"number"`
}

func TestAppendAndReadStreamPreservesOrderAndContent(t *testing.T) {
	store := events.NewEventStore(events.NewMemoryRepository())
	aggregateID := uuid.New()
	for i := 1; i <= 5; i++ {
		req := events.AppendRequest{
			AggregateType: events.AggregateAccount,
			AggregateID:   aggregateID,
			Sequence:      int64(i),
			EventType:     events.AccountCreated,
			EventVersion:  1,
			Actor:         "test-suite",
			Payload:       testPayload{Number: i},
		}
		_, err := store.Append(context.Background(), req)
		if err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}
	stream, err := store.ReadStream(context.Background(), events.AggregateAccount, aggregateID)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if len(stream) != 5 {
		t.Fatalf("expected 5 events, got %d", len(stream))
	}
	for i, event := range stream {
		isInvalid := event.Sequence != int64(i+1) ||
			event.AggregateID != aggregateID ||
			event.EventType != events.AccountCreated ||
			event.EventVersion != 1 ||
			event.Actor != "test-suite" ||
			event.EventID.Version() != 7 ||
			event.OccurredAt.Location() != time.UTC

		if isInvalid {
			t.Fatalf("unexpected envelope at position %d: %+v", i, event)
		}
		var payload testPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.Number != i+1 {
			t.Fatalf("unexpected payload at %d: %+v, %v", i, payload, err)
		}
	}
}

func TestAppendRejectsStaleOrDuplicateSequence(t *testing.T) {
	store := events.NewEventStore(events.NewMemoryRepository())
	req := events.AppendRequest{
		AggregateType: events.AggregateAccount,
		AggregateID:   uuid.New(),
		Sequence:      1,
		EventType:     events.AccountCreated,
		EventVersion:  1,
		Actor:         "test",
		Payload:       testPayload{Number: 1},
	}
	if _, err := store.Append(context.Background(), req); err != nil {
		t.Fatalf("initial append: %v", err)
	}
	if _, err := store.Append(context.Background(), req); !errors.Is(err, events.ErrSequenceConflict) {
		t.Fatalf("expected sequence conflict, got %v", err)
	}
}
