package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestCheckOrReservePreventsDuplicateSideEffects(t *testing.T) {
	store := NewStore()
	ctx := context.Background()
	hash := HashRequest([]byte(`{"currency":"USD"}`))
	sideEffects := 0
	first, err := store.CheckOrReserve(ctx, "request-123", "account.create", hash)
	if err != nil || !first.ShouldProceed() {
		t.Fatalf("first request should proceed: %+v, %v", first, err)
	}
	sideEffects++
	if err := store.StoreResponse(ctx, *first.ProceedToken, map[string]string{"account_id": "acc_123"}); err != nil {
		t.Fatalf("store response: %v", err)
	}
	second, err := store.CheckOrReserve(ctx, "request-123", "account.create", hash)
	if err != nil || second.ShouldProceed() {
		t.Fatalf("duplicate should replay: %+v, %v", second, err)
	}
	if sideEffects != 1 {
		t.Fatalf("expected one side effect, got %d", sideEffects)
	}
	var response map[string]string
	if err := json.Unmarshal(second.CachedResponse, &response); err != nil || response["account_id"] != "acc_123" {
		t.Fatalf("unexpected response: %s, %v", second.CachedResponse, err)
	}
}

func TestCheckOrReserveRejectsDifferentRequest(t *testing.T) {
	store := NewStore()
	if _, err := store.CheckOrReserve(context.Background(), "request-123", "account.create", "hash-one"); err != nil {
		t.Fatal(err)
	}
	_, err := store.CheckOrReserve(context.Background(), "request-123", "account.create", "hash-two")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}
