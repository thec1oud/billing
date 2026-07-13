package idempotency

import "errors"

var (
	ErrConflict   = errors.New("IDEMPOTENCY_CONFLICT")
	ErrInProgress = errors.New("idempotent operation is in progress")
	ErrNotOwner   = errors.New("idempotency proceed token does not own the reservation")
	ErrInvalidKey = errors.New("idempotency key, operation type, and request hash are required")
)
