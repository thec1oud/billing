package tests

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/eventstore"
	"github.com/thec1oud/billing/internal/projection"
	sharedEvents "github.com/thec1oud/billing/internal/shared/events"
)

type testPayload struct {
	Number int `json:"number"`
}

func TestEventStoreAppendAndReadStreamPreservesOrderAndContent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := eventstore.New(eventstore.NewMemoryRepository())
	aggregateID := uuid.New()

	for i := 1; i <= 5; i++ {
		_, err := store.Append(ctx, eventstore.AppendRequest{
			AggregateType: sharedEvents.AggregateAccount,
			AggregateID:   aggregateID,
			Sequence:      int64(i),
			EventType:     sharedEvents.AccountCreated,
			EventVersion:  1,
			Actor:         "test-suite",
			Payload:       testPayload{Number: i},
		})
		if err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}

	stream, err := store.ReadStream(ctx, sharedEvents.AggregateAccount, aggregateID)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if len(stream) != 5 {
		t.Fatalf("expected 5 events, got %d", len(stream))
	}

	for i, event := range stream {
		if event.Sequence != int64(i+1) ||
			event.AggregateID != aggregateID ||
			event.AggregateType != sharedEvents.AggregateAccount ||
			event.EventType != sharedEvents.AccountCreated ||
			event.EventVersion != 1 ||
			event.Actor != "test-suite" ||
			event.EventID.Version() != 7 ||
			event.OccurredAt.Location() != time.UTC {
			t.Fatalf("unexpected envelope at position %d: %+v", i, event)
		}
		var payload testPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode payload at position %d: %v", i, err)
		}
		if !reflect.DeepEqual(payload, testPayload{Number: i + 1}) {
			t.Fatalf("unexpected payload at position %d: %+v", i, payload)
		}
	}
}

func TestEventStoreRejectsStaleOrDuplicateSequence(t *testing.T) {
	t.Parallel()
	store := eventstore.New(eventstore.NewMemoryRepository())
	aggregateID := uuid.New()
	req := eventstore.AppendRequest{AggregateType: sharedEvents.AggregateAccount, AggregateID: aggregateID, Sequence: 1, EventType: sharedEvents.AccountCreated, EventVersion: 1, Actor: "test", Payload: testPayload{Number: 1}}
	if _, err := store.Append(context.Background(), req); err != nil {
		t.Fatalf("initial append: %v", err)
	}
	if _, err := store.Append(context.Background(), req); !errors.Is(err, eventstore.ErrSequenceConflict) {
		t.Fatalf("expected sequence conflict, got %v", err)
	}
}

func TestRebuildCounterFromEvents(t *testing.T) {
	t.Parallel()
	events := []sharedEvents.Event{
		{EventType: "counter.incremented", Payload: json.RawMessage(`{"amount": 7}`)},
		{EventType: "counter.decremented", Payload: json.RawMessage(`{"amount": 2}`)},
		{EventType: "counter.incremented", Payload: json.RawMessage(`{"amount": 4}`)},
	}

	state, err := projection.Rebuild(0, events, func(current int, event sharedEvents.Event) (int, error) {
		var payload struct {
			Amount int `json:"amount"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return current, err
		}
		switch event.EventType {
		case "counter.incremented":
			return current + payload.Amount, nil
		case "counter.decremented":
			return current - payload.Amount, nil
		default:
			return current, nil
		}
	})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if state != 9 {
		t.Fatalf("expected counter state 9, got %d", state)
	}
}
