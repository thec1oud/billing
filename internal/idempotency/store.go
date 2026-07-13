// Package idempotency prevents a client retry from executing a mutating
// operation more than once.
package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const MinimumRetention = 24 * time.Hour

var (
	ErrConflict   = errors.New("IDEMPOTENCY_CONFLICT")
	ErrInProgress = errors.New("idempotent operation is in progress")
	ErrNotOwner   = errors.New("idempotency proceed token does not own the reservation")
	ErrInvalidKey = errors.New("idempotency key, operation type, and request hash are required")
)

// ProceedToken proves that its holder owns a reservation and may cache the
// response. 
type ProceedToken struct {
	key   string
	value string
}

// Decision is the outcome of CheckOrReserve. Exactly one of ProceedToken and
// CachedResponse is set when err is nil.
type Decision struct {
	ProceedToken   *ProceedToken
	CachedResponse json.RawMessage
}

func (d Decision) ShouldProceed() bool { return d.ProceedToken != nil }

type record struct {
	OperationType string
	RequestHash   string
	Token         string
	Response      json.RawMessage
	Completed     bool
	ExpiresAt     time.Time
}


type Store struct {
	mu      sync.Mutex
	records map[string]record
	ttl     time.Duration
	now     func() time.Time
	newID   func() (uuid.UUID, error)
}

func NewStore() *Store {
	return NewStoreWithRetention(MinimumRetention)
}

func NewStoreWithRetention(retention time.Duration) *Store {
	if retention < MinimumRetention {
		retention = MinimumRetention
	}
	return &Store{
		records: make(map[string]record),
		ttl:     retention,
		now:     func() time.Time { return time.Now().UTC() },
		newID:   uuid.NewV7,
	}
}

func (s *Store) CheckOrReserve(_ context.Context, key, operationType, requestHash string) (Decision, error) {
	if key == "" || operationType == "" || requestHash == "" {
		return Decision{}, ErrInvalidKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.records[key]; ok {
		if !s.now().Before(existing.ExpiresAt) {
			delete(s.records, key)
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

	id, err := s.newID()
	if err != nil {
		return Decision{}, fmt.Errorf("create idempotency proceed token: %w", err)
	}
	token := ProceedToken{key: key, value: id.String()}
	s.records[key] = record{
		OperationType: operationType,
		RequestHash:   requestHash,
		Token:         token.value,
		ExpiresAt:     s.now().Add(s.ttl),
	}
	return Decision{ProceedToken: &token}, nil
}

// StoreResponse marks a reservation complete. The response is JSON encoded so
// callers can replay the same result without reconstructing side effects.
func (s *Store) StoreResponse(_ context.Context, token ProceedToken, response any) error {
	if token.key == "" || token.value == "" {
		return ErrNotOwner
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.records[token.key]
	if !ok || !s.now().Before(existing.ExpiresAt) || existing.Token != token.value {
		return ErrNotOwner
	}
	existing.Response = encoded
	existing.Completed = true
	s.records[token.key] = existing
	return nil
}


func HashRequest(request []byte) string {
	hash := sha256.Sum256(request)
	return hex.EncodeToString(hash[:])
}
