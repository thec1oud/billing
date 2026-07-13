package money

import "errors"

var (
	ErrInvalidCurrency  = errors.New("currency must be a three-letter ISO 4217 code")
	ErrCurrencyMismatch = errors.New("cannot operate on money with different currencies")
	ErrDivisionByZero   = errors.New("money multiplier denominator must not be zero")
	ErrOverflow         = errors.New("money amount exceeds int64 range")
)
