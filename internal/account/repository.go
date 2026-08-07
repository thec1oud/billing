package account

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	events "github.com/thec1oud/billing/internal/shared/eventstore"
)

// Repository is responsible for persisting Account events
// and rebuilding Account aggregates from the event store.
//
// The repository does not contain business state-transition rules.
// Those rules belong to the Account aggregate and Service.
type Repository struct {
	store events.EventStore
}

// NewRepository creates a new Account repository.
func NewRepository(store events.EventStore) *Repository {
	return &Repository{
		store: store,
	}
}

// Append stores one Account event in the event store.
//
// The service is responsible for constructing the correct
// AppendRequest, including aggregate ID and sequence number.
func (r *Repository) Append(
	ctx context.Context,
	req events.AppendRequest,
) error {
	if r == nil || r.store == nil {
		return errors.New("account repository: event store is not configured")
	}

	if req.AggregateType != events.AggregateAccount {
		return fmt.Errorf(
			"account repository: invalid aggregate type %q",
			req.AggregateType,
		)
	}

	if req.AggregateID == "" {
		return errors.New(
			"account repository: aggregate id cannot be empty",
		)
	}

	if req.Sequence <= 0 {
		return errors.New(
			"account repository: sequence must be greater than zero",
		)
	}

	if req.EventType == "" {
		return errors.New(
			"account repository: event type cannot be empty",
		)
	}

	_, err := r.store.Append(ctx, req)
	if err != nil {
		return fmt.Errorf(
			"account repository: append event %s: %w",
			req.EventType,
			err,
		)
	}

	return nil
}

// Get retrieves an Account aggregate by ID.
//
// The account is rebuilt by replaying its complete event stream.
// The event stream is authoritative; the Account object is a
// derived representation of that history.
func (r *Repository) Get(
	ctx context.Context,
	accountID uuid.UUID,
) (*Account, error) {
	if r == nil || r.store == nil {
		return nil, errors.New(
			"account repository: event store is not configured",
		)
	}

	if accountID == uuid.Nil {
		return nil, errors.New(
			"account repository: account id cannot be empty",
		)
	}

	// The event store currently accepts aggregate IDs as strings.
	//
	// The Account aggregate itself continues to use uuid.UUID.
	stream, err := r.store.ReadStream(
		ctx,
		events.AggregateAccount,
		accountID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"account repository: read account stream: %w",
			err,
		)
	}

	// An empty event stream means the aggregate does not exist.
	if len(stream) == 0 {
		return nil, fmt.Errorf(
			"%w: %s",
			ErrAccountNotFound,
			accountID,
		)
	}

	account, err := Rebuild(stream)
	if err != nil {
		return nil, fmt.Errorf(
			"account repository: rebuild account %s: %w",
			accountID,
			err,
		)
	}

	// Extra integrity check.
	//
	// The event stream we just replayed must produce the account
	// that the caller requested.
	if account.AccountID != accountID {
		return nil, fmt.Errorf(
			"%w: requested=%s rebuilt=%s",
			ErrAccountIDMismatch,
			accountID,
			account.AccountID,
		)
	}

	return account, nil
}
