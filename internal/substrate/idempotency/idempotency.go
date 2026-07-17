package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// Enterprise Idempotency Specification Errors (RFC §17.3 compliant)
var (
	ErrConflict   = errors.New("IDEMPOTENCY_CONFLICT: operation fingerprint mismatch detected")
	ErrInProgress = errors.New("idempotent operation is in progress: lease lock held by another worker")
	ErrNotOwner   = errors.New("idempotency proceed token error: worker context does not own this reservation")
	ErrInvalidKey = errors.New("idempotency error: key, operation type, and request hash parameters are required")
)

// ProceedToken acts as a cryptographic proof-of-ownership lease for a reservation slot
type ProceedToken struct {
	Key   string
	Value string
}

// Decision establishes authorization to execute a mutating database operation
type Decision struct {
	ProceedToken   *ProceedToken
	CachedResponse json.RawMessage
}

func (d Decision) ShouldProceed() bool { return d.ProceedToken != nil }

// Tracker provides the public API contract for Track B and C systems
type Tracker interface {
	CheckOrReserve(ctx context.Context, key, operationType, requestHash string) (Decision, error)
	StoreResponse(ctx context.Context, token ProceedToken, response any) error
}

// Execute wraps any domain logic action callback with automatic distributed idempotency handling.
// It abstracts the entire check -> run -> store lifecycle for upstream developers.
func Execute[T any](
	ctx context.Context,
	tracker Tracker,
	key, operationType, requestHash string,
	action func() (T, error),
) (T, error) {
	var zero T
	decision, err := tracker.CheckOrReserve(ctx, key, operationType, requestHash)
	if err != nil {
		return zero, err
	}
	
	// If a response is already cached, bypass execution entirely
	if !decision.ShouldProceed() {
		if err := json.Unmarshal(decision.CachedResponse, &zero); err != nil {
			return zero, fmt.Errorf("unmarshal cached response: %w", err)
		}
		return zero, nil
	}

	// Execute the core enterprise action side-effects
	result, err := action()
	if err != nil {
		return zero, err
	}

	// Commit the successful calculation result into the safe cache layer
	if err := tracker.StoreResponse(ctx, *decision.ProceedToken, result); err != nil {
		return zero, fmt.Errorf("store idempotent response: %w", err)
	}
	
	return result, nil
}

// HashRequest provides a fast utility function generating sha256 fingerprinted inputs
func HashRequest(request []byte) string {
	hash := sha256.Sum256(request)
	return hex.EncodeToString(hash[:])
}
