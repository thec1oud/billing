package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestCheckOrReservePreventsDuplicateSideEffects(t *testing.T) {
	t.Parallel()
	store := NewStore()
	ctx := context.Background()
	requestHash := HashRequest([]byte(`{"currency":"USD"}`))
	sideEffects := 0

	first, err := store.CheckOrReserve(ctx, "request-123", "account.create", requestHash)
	if err != nil || !first.ShouldProceed() {
		t.Fatalf("first request should proceed, decision=%+v err=%v", first, err)
	}
	sideEffects++
	response := map[string]string{"account_id": "acc_123"}
	if err := store.StoreResponse(ctx, *first.ProceedToken, response); err != nil {
		t.Fatalf("store response: %v", err)
	}

	second, err := store.CheckOrReserve(ctx, "request-123", "account.create", requestHash)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	if second.ShouldProceed() {
		t.Fatal("duplicate request must replay its response, not proceed")
	}
	if sideEffects != 1 {
		t.Fatalf("expected exactly one side effect, got %d", sideEffects)
	}

	var replayed map[string]string
	if err := json.Unmarshal(second.CachedResponse, &replayed); err != nil {
		t.Fatalf("decode cached response: %v", err)
	}
	if replayed["account_id"] != "acc_123" {
		t.Fatalf("unexpected cached response: %s", second.CachedResponse)
	}
}

func TestCheckOrReserveRejectsReusedKeyForDifferentRequest(t *testing.T) {
	t.Parallel()
	store := NewStore()
	_, err := store.CheckOrReserve(context.Background(), "request-123", "account.create", "hash-one")
	if err != nil {
		t.Fatalf("reserve request: %v", err)
	}

	_, err = store.CheckOrReserve(context.Background(), "request-123", "account.create", "hash-two")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected IDEMPOTENCY_CONFLICT, got %v", err)
	}
}

func TestCheckOrReserveReportsInProgressDuplicate(t *testing.T) {
	t.Parallel()
	store := NewStore()
	_, err := store.CheckOrReserve(context.Background(), "request-123", "account.create", "hash-one")
	if err != nil {
		t.Fatalf("reserve request: %v", err)
	}

	_, err = store.CheckOrReserve(context.Background(), "request-123", "account.create", "hash-one")
	if !errors.Is(err, ErrInProgress) {
		t.Fatalf("expected in-progress error, got %v", err)
	}
}
