package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const MinimumRetention = 24 * time.Hour

type record struct {
	OperationType string
	RequestHash   string
	Token         string
	Response      json.RawMessage
	Completed     bool
	ExpiresAt     time.Time
}

type MemoryIdempotencyStore struct {
	mu      sync.RWMutex
	records map[string]record
	ttl     time.Duration
	now     func() time.Time
	newID   func() (uuid.UUID, error)
}

func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return NewMemoryIdempotencyStoreWithTTL(MinimumRetention)
}

func NewMemoryIdempotencyStoreWithTTL(retention time.Duration) *MemoryIdempotencyStore {
	if retention < MinimumRetention {
		retention = MinimumRetention
	}
	return &MemoryIdempotencyStore{
		records: make(map[string]record),
		ttl:     retention,
		now:     func() time.Time { return time.Now().UTC() },
		newID:   uuid.NewV7,
	}
}

type Store = MemoryIdempotencyStore

func NewStore() *Store {
	return NewMemoryIdempotencyStore()
}

func (m *MemoryIdempotencyStore) CheckOrReserve(ctx context.Context, key, operationType, requestHash string) (Decision, error) {
	if key == "" || operationType == "" || requestHash == "" {
		return Decision{}, ErrInvalidKey
	}

	m.mu.Lock()
	// Using a defer Unlock strategy ensures thread safety across recursive loops or quick escapes
	defer m.mu.Unlock()

	existing, found := m.records[key]
	if found {
		if !m.now().Before(existing.ExpiresAt) {
			delete(m.records, key)
		} else {
			if existing.OperationType != operationType || existing.RequestHash != requestHash {
				return Decision{}, ErrConflict
			}
			if !existing.Completed {
				return Decision{}, ErrInProgress
			}
			return Decision{CachedResponse: append(json.RawMessage(nil), existing.Response...)}, nil
		}
	}

	// Generate lease reservation tokens natively inside the concurrency lock boundary
	id, err := m.newID()
	if err != nil {
		return Decision{}, fmt.Errorf("create idempotency token: %w", err)
	}

	token := ProceedToken{Key: key, Value: id.String()}
	m.records[key] = record{
		OperationType: operationType,
		RequestHash:   requestHash,
		Token:         token.Value,
		ExpiresAt:     m.now().Add(m.ttl),
	}

	return Decision{ProceedToken: &token}, nil
}

func (m *MemoryIdempotencyStore) StoreResponse(ctx context.Context, token ProceedToken, response any) error {
	if token.Key == "" || token.Value == "" {
		return ErrNotOwner
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	value, exists := m.records[token.Key]
	if !exists || value.Token != token.Value {
		return ErrNotOwner
	}

	value.Response = encoded
	value.Completed = true
	m.records[token.Key] = value

	return nil
}
