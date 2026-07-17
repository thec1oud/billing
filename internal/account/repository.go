package account

import (
	"context"

	"github.com/google/uuid"

	events "github.com/thec1oud/billing/internal/substrate/eventstore"
)

// Repository is responsible for persisting and rebuilding
// Account aggregates from the event store.
type Repository struct {
	store events.EventStore
}

// NewRepository creates a new Account repository.
func NewRepository(store events.EventStore) *Repository {
	return &Repository{
		store: store,
	}
}

// Append stores a single Account event in the event store.
func (r *Repository) Append(
	ctx context.Context,
	req events.AppendRequest,
) error {

	_, err := r.store.Append(ctx, req)
	return err
}

// Get rebuilds an Account aggregate by replaying its event stream.
func (r *Repository) Get(
	ctx context.Context,
	accountID uuid.UUID,
) (*Account, error) {

	stream, err := r.store.ReadStream(
		ctx,
		events.AggregateAccount,
		accountID,
	)
	if err != nil {
		return nil, err
	}

	account, err := Rebuild(stream)
	if err != nil {
		return nil, err
	}

	return account, nil
}
