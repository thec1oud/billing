package events

import "errors"

var (
	ErrSequenceConflict = errors.New("event sequence conflict")
	ErrAppendFailed     = errors.New("failed to append event")
	ErrReadFailed       = errors.New("failed to read event stream")
	ErrInvalidSequence  = errors.New("event sequence must be positive")
)
