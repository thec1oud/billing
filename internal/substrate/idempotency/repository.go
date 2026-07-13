package idempotency

import (
	"context"
	"sync"
)


type Repository interface {
	Get(ctx context.Context, key string) (record, bool, error)
	Reserve(ctx context.Context, key string, value record) (bool, error)
	Complete(ctx context.Context, key, token string, response []byte) (bool, error)
	Delete(ctx context.Context, key string) error
}


type MemoryRepository struct {
	mu      sync.Mutex
	records map[string]record
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{records: make(map[string]record)}
}

func (r *MemoryRepository) Get(_ context.Context, key string) (record, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.records[key]
	value.Response = append([]byte(nil), value.Response...)
	return value, ok, nil
}

func (r *MemoryRepository) Reserve(_ context.Context, key string, value record) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.records[key]; exists {
		return false, nil
	}
	r.records[key] = value
	return true, nil
}

func (r *MemoryRepository) Complete(_ context.Context, key, token string, response []byte) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, exists := r.records[key]
	if !exists || value.Token != token {
		return false, nil
	}
	value.Response = append([]byte(nil), response...)
	value.Completed = true
	r.records[key] = value
	return true, nil
}

func (r *MemoryRepository) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.records, key)
	return nil
}
