package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
)

func Execute[T any](ctx context.Context, store *Store, key, operationType, requestHash string, action func() (T, error)) (T, error) {
	var zero T
	decision, err := store.CheckOrReserve(ctx, key, operationType, requestHash)
	if err != nil {
		return zero, err
	}
	if !decision.ShouldProceed() {
		if err := json.Unmarshal(decision.CachedResponse, &zero); err != nil {
			return zero, fmt.Errorf("unmarshal cached response: %w", err)
		}
		return zero, nil
	}

	result, err := action()
	if err != nil {
		return zero, err
	}
	if err := store.StoreResponse(ctx, *decision.ProceedToken, result); err != nil {
		return zero, fmt.Errorf("store idempotent response: %w", err)
	}
	return result, nil
}
