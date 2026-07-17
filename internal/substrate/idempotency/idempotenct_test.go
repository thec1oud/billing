package idempotency

import (
	"context"
	"encoding/json"
	"testing"
)

func TestA3_IdempotencyLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryIdempotencyStore()

	key := "test_key_001"
	operation := "CreateInvoice"
	hash := HashRequest([]byte(`{"amount":100}`))

	dec1, err := store.CheckOrReserve(ctx, key, operation, hash)
	if err != nil || !dec1.ShouldProceed() {
		t.Fatalf("First pass should proceed, got error: %v", err)
	}

	_, errInProgress := store.CheckOrReserve(ctx, key, operation, hash)
	if errInProgress != ErrInProgress {
		t.Fatalf("Expected ErrInProgress, got: %v", errInProgress)
	}

	type Resp struct{ Status string }
	if err := store.StoreResponse(ctx, *dec1.ProceedToken, Resp{Status: "SUCCESS"}); err != nil {
		t.Fatalf("store response failed: %v", err)
	}

	mockAction := func() (Resp, error) {
		return Resp{Status: "SUCCESS"}, nil
	}

	res, err := Execute(ctx, store, key, operation, hash, mockAction)
	if err != nil || res.Status != "SUCCESS" {
		t.Fatalf("Execute failed: %v", err)
	}

	dec2, err := store.CheckOrReserve(ctx, key, operation, hash)
	if err != nil || dec2.ShouldProceed() {
		t.Fatalf("Second pass should not proceed, got error: %v", err)
	}

	var cachedResp Resp
	if err := json.Unmarshal(dec2.CachedResponse, &cachedResp); err != nil || cachedResp.Status != "SUCCESS" {
		t.Fatalf("Failed to decode cached output: %v", err)
	}

	wrongHash := HashRequest([]byte(`{"amount":200}`))
	_, errConflict := store.CheckOrReserve(ctx, key, operation, wrongHash)
	if errConflict != ErrConflict {
		t.Fatalf("Expected ErrConflict, got: %v", errConflict)
	}
}
