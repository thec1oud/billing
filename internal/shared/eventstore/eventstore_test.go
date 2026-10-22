package events_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventrepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

func TestPostgresEventStore_SequenceGeneration(t *testing.T) {
	pool, err := testutil.GetTestPool()
	if err != nil {
		t.Skipf("Skipping integration test: failed to get test pool (%v)", err)
	}
	defer pool.Close()

	ctx := context.Background()
	store := eventrepo.NewPostgresEventStore(pool)

	aggID := "inv_" + uuid.NewString()
	aggType := eventmodel.AggregateInvoice
	actor := eventmodel.Actor{Type: "USER", ID: "test-runner"}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	e1, err := store.Append(ctx, tx, eventmodel.AppendRequest{
		AggregateType: aggType,
		AggregateID:   aggID,
		EventType:     eventmodel.InvoiceCreated,
		EventVersion:  1,
		Actor:         actor,
		Payload:       map[string]any{"total": 1000},
	})
	if err != nil {
		t.Fatalf("failed to append first event: %v", err)
	}
	if e1.Sequence <= 0 {
		t.Errorf("expected positive sequence, got %d", e1.Sequence)
	}

	e2, err := store.Append(ctx, tx, eventmodel.AppendRequest{
		AggregateType: aggType,
		AggregateID:   aggID,
		EventType:     eventmodel.InvoiceFinalized,
		EventVersion:  1,
		Actor:         actor,
		Payload:       map[string]any{"status": "finalized"},
	})
	if err != nil {
		t.Fatalf("failed to append second event: %v", err)
	}
	if e2.Sequence <= e1.Sequence {
		t.Errorf("expected e2.Sequence (%d) > e1.Sequence (%d)", e2.Sequence, e1.Sequence)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	events, err := store.ReadStream(ctx, aggType, aggID)
	if err != nil {
		t.Fatalf("failed to read stream: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events in stream, got %d", len(events))
	}
	if events[0].Sequence != e1.Sequence || events[1].Sequence != e2.Sequence {
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
	store := eventrepo.NewPostgresEventStore(pool)

	aggType := eventmodel.AggregateType("ACCOUNT")
	actor := eventmodel.Actor{Type: "USER", ID: "test-runner"}

	aggID1 := "acc_" + uuid.NewString()
	aggID2 := "acc_" + uuid.NewString()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	e1, err := store.Append(ctx, tx, eventmodel.AppendRequest{
		AggregateType: aggType,
		AggregateID:   aggID1,
		EventType:     eventmodel.EventType("AccountCreated"),
		EventVersion:  1,
		Actor:         actor,
		Payload:       map[string]any{"name": "Org A"},
	})
	if err != nil {
		t.Fatalf("failed to append to agg1: %v", err)
	}

	e2, err := store.Append(ctx, tx, eventmodel.AppendRequest{
		AggregateType: aggType,
		AggregateID:   aggID2,
		EventType:     eventmodel.EventType("AccountCreated"),
		EventVersion:  1,
		Actor:         actor,
		Payload:       map[string]any{"name": "Org B"},
	})
	if err != nil {
		t.Fatalf("failed to append to agg2: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	if e1.Sequence <= 0 || e2.Sequence <= 0 {
		t.Errorf("expected positive sequences, got %d and %d", e1.Sequence, e2.Sequence)
	}
	if e2.Sequence <= e1.Sequence {
		t.Errorf("expected e2.Sequence (%d) > e1.Sequence (%d)", e2.Sequence, e1.Sequence)
	}

	stream1, err := store.ReadStream(ctx, aggType, aggID1)
	if err != nil {
		t.Fatalf("failed to read stream 1: %v", err)
	}
	if len(stream1) != 1 || stream1[0].Sequence != e1.Sequence {
		t.Errorf("expected stream 1 to contain e1 (%d), got %+v", e1.Sequence, stream1)
	}

	stream2, err := store.ReadStream(ctx, aggType, aggID2)
	if err != nil {
		t.Fatalf("failed to read stream 2: %v", err)
	}
	if len(stream2) != 1 || stream2[0].Sequence != e2.Sequence {
		t.Errorf("expected stream 2 to contain e2 (%d), got %+v", e2.Sequence, stream2)
	}
}
