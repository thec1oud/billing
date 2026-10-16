package events_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/google/uuid"

	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventrepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

// TestPostgresEventStore_ConcurrentAppend verifies that concurrent appends to the
// same aggregate stream are serialized correctly by the advisory lock introduced in
// the fix for Issue #34.
//
// Before the fix: two goroutines racing on MAX(sequence)+1 would read the same
// MAX, one would fail on the unique constraint, and the business operation would
// surface an ErrSequenceConflict with no retry.
//
// After the fix: pg_advisory_xact_lock serializes writers; every goroutine gets a
// unique, gapless sequence number and all N events are present in the stream.
func TestPostgresEventStore_ConcurrentAppend(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)

	const workers = 20

	ctx := context.Background()
	store := eventrepo.NewPostgresEventStore(pool)

	aggID := "acc_" + uuid.NewString()
	aggType := eventmodel.AggregateAccount
	actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: "concurrency-test"}

	type result struct {
		seq int64
		err error
	}

	results := make(chan result, workers)
	var wg sync.WaitGroup

	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// Each goroutine opens its own transaction — the realistic scenario
			// where N concurrent HTTP requests all try to append to the same aggregate.
			tx, err := pool.Begin(ctx)
			if err != nil {
				results <- result{err: fmt.Errorf("worker %d: begin tx: %w", i, err)}
				return
			}

			evt, err := store.Append(ctx, tx, eventmodel.AppendRequest{
				AggregateType: aggType,
				AggregateID:   aggID,
				EventType:     eventmodel.AccountCreated,
				EventVersion:  1,
				Actor:         actor,
				Payload:       map[string]any{"worker": i},
			})
			if err != nil {
				tx.Rollback(ctx) //nolint:errcheck
				results <- result{err: fmt.Errorf("worker %d: append: %w", i, err)}
				return
			}

			if err := tx.Commit(ctx); err != nil {
				results <- result{err: fmt.Errorf("worker %d: commit: %w", i, err)}
				return
			}

			results <- result{seq: evt.Sequence}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var sequences []int64
	for r := range results {
		if r.err != nil {
			t.Errorf("concurrent append failed: %v", r.err)
			continue
		}
		sequences = append(sequences, r.seq)
	}

	if t.Failed() {
		return
	}

	if len(sequences) != workers {
		t.Fatalf("expected %d successful appends, got %d", workers, len(sequences))
	}

	sort.Slice(sequences, func(i, j int) bool { return sequences[i] < sequences[j] })

	// Gapless sequence 1..N — no conflicts, no duplicates, no gaps.
	for i, seq := range sequences {
		want := int64(i + 1)
		if seq != want {
			t.Errorf(
				"sequence gap or duplicate at position %d: got %d, want %d (full list: %v)",
				i, seq, want, sequences,
			)
			break
		}
	}

	// Cross-check: the DB stream must contain exactly N events in order.
	stream, err := store.ReadStream(ctx, aggType, aggID)
	if err != nil {
		t.Fatalf("ReadStream failed: %v", err)
	}
	if len(stream) != workers {
		t.Fatalf("ReadStream returned %d events, expected %d", len(stream), workers)
	}
	for i, evt := range stream {
		if evt.Sequence != int64(i+1) {
			t.Errorf("stream event[%d].Sequence = %d, want %d", i, evt.Sequence, i+1)
		}
	}
}
