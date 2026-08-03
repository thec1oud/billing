package events_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/eventstore/repository"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

func TestPostgresEventStore_SequenceGeneration(t *testing.T) {
	pool, err := testutil.GetTestPool()
	if err != nil {
		t.Skipf("Skipping integration test: failed to get test pool (%v)", err)
	}
	defer pool.Close()

	ctx := context.Background()
	store := repository.NewPostgresEventStore(pool)

	aggID := "inv_" + uuid.NewString()
	aggType := model.AggregateType("INVOICE")
	actor := json.RawMessage(`{"user":"test-runner"}`)

	e1, err := store.Append(ctx, model.AppendRequest{
		AggregateType: aggType,
		AggregateID:   aggID,
		EventType:     model.EventType("InvoiceCreated"),
		EventVersion:  1,
		Actor:         actor,
		Payload:       map[string]any{"total": 1000},
	})
	if err != nil {
		t.Fatalf("failed to append first event: %v", err)
	}
	if e1.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", e1.Sequence)
	}

	e2, err := store.Append(ctx, model.AppendRequest{
		AggregateType: aggType,
		AggregateID:   aggID,
		EventType:     model.EventType("InvoiceFinalized"),
		EventVersion:  1,
		Actor:         actor,
		Payload:       map[string]any{"status": "finalized"},
	})
	if err != nil {
		t.Fatalf("failed to append second event: %v", err)
	}
	if e2.Sequence != 2 {
		t.Errorf("expected sequence 2, got %d", e2.Sequence)
	}

	events, err := store.ReadStream(ctx, aggType, aggID)
	if err != nil {
		t.Fatalf("failed to read stream: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events in stream, got %d", len(events))
	}
	if events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Errorf("stream returned out-of-order sequences: %d, %d", events[0].Sequence, events[1].Sequence)
	}
}

func TestPostgresEventStore_SequenceIsolation(t *testing.T) {
	pool, err := testutil.GetTestPool()
	if err != nil {
		t.Skipf("Skipping integration test: failed to get test pool (%v)", err)
	}
	defer pool.Close()

	ctx := context.Background()
	store := repository.NewPostgresEventStore(pool)

	aggType := model.AggregateType("ACCOUNT")
	actor := json.RawMessage(`{"user":"test-runner"}`)

	aggID1 := "acc_" + uuid.NewString()
	aggID2 := "acc_" + uuid.NewString()

	e1, err := store.Append(ctx, model.AppendRequest{
		AggregateType: aggType,
		AggregateID:   aggID1,
		EventType:     model.EventType("AccountCreated"),
		EventVersion:  1,
		Actor:         actor,
		Payload:       map[string]any{"name": "Org A"},
	})
	if err != nil {
		t.Fatalf("failed to append to agg1: %v", err)
	}

	e2, err := store.Append(ctx, model.AppendRequest{
		AggregateType: aggType,
		AggregateID:   aggID2,
		EventType:     model.EventType("AccountCreated"),
		EventVersion:  1,
		Actor:         actor,
		Payload:       map[string]any{"name": "Org B"},
	})
	if err != nil {
		t.Fatalf("failed to append to agg2: %v", err)
	}

	if e1.Sequence != 1 || e2.Sequence != 1 {
		t.Errorf("expected both independent aggregates to have sequence = 1, got %d and %d", e1.Sequence, e2.Sequence)
	}
}
