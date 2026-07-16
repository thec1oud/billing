// Package idempotency prevents retried client commands from causing a second side effect.
package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const MinimumRetention = 24 * time.Hour

type Store struct {
	repository Repository
	ttl        time.Duration
	now        func() time.Time
	newID      func() (uuid.UUID, error)
}

// NewStore creates the in-memory  store.
func NewStore() *Store { return NewStoreWithRepository(NewMemoryRepository(), MinimumRetention) }

func NewStoreWithRepository(repository Repository, retention time.Duration) *Store {
	if retention < MinimumRetention {
		retention = MinimumRetention
	}
	return &Store{
		repository: repository,
		ttl:        retention,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      uuid.NewV7,
	}
}

func (s *Store) CheckOrReserve(ctx context.Context, key, operationType, requestHash string) (Decision, error) {
	if key == "" || operationType == "" || requestHash == "" {
		return Decision{}, ErrInvalidKey
	}

	existing, found, err := s.repository.Get(ctx, key)
	if err != nil {
		return Decision{}, err
	}
	if found {
		if !s.now().Before(existing.ExpiresAt) {
			if err := s.repository.Delete(ctx, key); err != nil {
				return Decision{}, err
			}
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
	reserved, err := s.repository.Reserve(ctx, key, record{
		OperationType: operationType,
		RequestHash:   requestHash,
		Token:         token.value,
		ExpiresAt:     s.now().Add(s.ttl),
	})
	if err != nil {
		return Decision{}, err
	}
	if !reserved {
		return s.CheckOrReserve(ctx, key, operationType, requestHash)
	}
	return Decision{ProceedToken: &token}, nil
}

func (s *Store) StoreResponse(ctx context.Context, token ProceedToken, response any) error {
	if token.key == "" || token.value == "" {
		return ErrNotOwner
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	stored, err := s.repository.Complete(ctx, token.key, token.value, encoded)
	if err != nil {
		return err
	}
	if !stored {
		return ErrNotOwner
	}
	return nil
}

func HashRequest(request []byte) string {
	hash := sha256.Sum256(request)
	return hex.EncodeToString(hash[:])
}
