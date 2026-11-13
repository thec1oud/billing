package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies an application error for HTTP mapping and error handling.
type Kind string

const (
	KindValidation   Kind = "VALIDATION"
	KindNotFound     Kind = "NOT_FOUND"
	KindConflict     Kind = "CONFLICT"
	KindUnauthorized Kind = "UNAUTHORIZED"
	KindForbidden    Kind = "FORBIDDEN"
	KindInternal     Kind = "INTERNAL"
)

// AppError is an interface for errors that carry classification kind, machine-readable code,
// and a client-safe message.
type AppError interface {
	error
	Kind() Kind
	Code() string
	SafeMessage() string
}

// Error represents a structured application error.
type Error struct {
	kind    Kind
	code    string
	message string
	err     error
}

// Error implements the standard error interface.
func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.message, e.err)
	}
	return e.message
}

// Unwrap returns the underlying error, enabling errors.Is and errors.As.
func (e *Error) Unwrap() error {
	return e.err
}

// Kind returns the error kind.
func (e *Error) Kind() Kind {
	return e.kind
}

// Code returns the machine-readable error code.
func (e *Error) Code() string {
	if e.code != "" {
		return e.code
	}
	switch e.kind {
	case KindValidation:
		return "invalid_request"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindUnauthorized:
		return "unauthorized"
	case KindForbidden:
		return "forbidden"
	default:
		return "internal_error"
	}
}

// SafeMessage returns the client-safe error message.
func (e *Error) SafeMessage() string {
	return e.message
}

// Is supports comparison with target errors.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	if e == t {
		return true
	}
	if e.code != "" && t.code != "" {
		return e.kind == t.kind && e.code == t.code
	}
	return e.kind == t.kind
}

// New creates a new application error.
func New(kind Kind, code, message string) *Error {
	return &Error{
		kind:    kind,
		code:    code,
		message: message,
	}
}

// Wrap wraps an existing error with a kind, code, and safe message.
func Wrap(err error, kind Kind, code, message string) *Error {
	return &Error{
		kind:    kind,
		code:    code,
		message: message,
		err:     err,
	}
}

// Validation creates a KindValidation error.
func Validation(code, message string) *Error {
	return New(KindValidation, code, message)
}

// NotFound creates a KindNotFound error.
func NotFound(code, message string) *Error {
	return New(KindNotFound, code, message)
}

// Conflict creates a KindConflict error.
func Conflict(code, message string) *Error {
	return New(KindConflict, code, message)
}

// Unauthorized creates a KindUnauthorized error.
func Unauthorized(code, message string) *Error {
	return New(KindUnauthorized, code, message)
}

// Forbidden creates a KindForbidden error.
func Forbidden(code, message string) *Error {
	return New(KindForbidden, code, message)
}

// Internal creates a KindInternal error wrapping an underlying error.
func Internal(err error, message string) *Error {
	return Wrap(err, KindInternal, "internal_error", message)
}

// AsAppError extracts an AppError from err if present.
func AsAppError(err error) (AppError, bool) {
	if err == nil {
		return nil, false
	}
	if ae, ok := errors.AsType[AppError](err); ok {
		return ae, true
	}
	return nil, false
}
