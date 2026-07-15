package plan

import (
	"context"
	"fmt"
	"sync"
)

type Repository struct {
	mu    sync.RWMutex
	plans map[string]map[int]Plan
}

func NewRepository() *Repository {
	return &Repository{
		plans: make(map[string]map[int]Plan),
	}
}

// Save always creates a NEW version.
// It never overwrites an existing version.
func (r *Repository) Save(
	_ context.Context,
	plan Plan,
) (Plan, error) {

	r.mu.Lock()
	defer r.mu.Unlock()

	versions, exists := r.plans[plan.ID]
	if !exists {
		versions = make(map[int]Plan)
		r.plans[plan.ID] = versions
	}

	if _, exists := versions[plan.Version]; exists {
		return Plan{}, fmt.Errorf(
			"plan %s version %d already exists",
			plan.ID,
			plan.Version,
		)
	}

	versions[plan.Version] = plan

	return plan, nil
}

// Get returns a specific version.
func (r *Repository) Get(
	_ context.Context,
	id string,
	version int,
) (Plan, error) {

	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, exists := r.plans[id]
	if !exists {
		return Plan{}, fmt.Errorf("plan %s not found", id)
	}

	plan, exists := versions[version]
	if !exists {
		return Plan{}, fmt.Errorf(
			"plan %s version %d not found",
			id,
			version,
		)
	}

	return plan, nil
}

// LatestVersion returns the highest version number.
func (r *Repository) LatestVersion(
	_ context.Context,
	id string,
) (int, error) {

	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, exists := r.plans[id]
	if !exists {
		return 0, fmt.Errorf("plan %s not found", id)
	}

	latest := 0

	for version := range versions {
		if version > latest {
			latest = version
		}
	}

	return latest, nil
}
