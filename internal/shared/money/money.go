// Package money provides integer-only monetary arithmetic for billing.
package money

import (
	"errors"
	"math/big"
	"strings"
)

var (
	ErrInvalidCurrency  = errors.New("currency must be a three-letter ISO 4217 code")
	ErrCurrencyMismatch = errors.New("cannot operate on money with different currencies")
	ErrDivisionByZero   = errors.New("money multiplier denominator must not be zero")
	ErrOverflow         = errors.New("money amount exceeds int64 range")
)

type Currency string

type Money struct {
	AmountMinor int64    `json:"amount_minor"`
	Currency    Currency `json:"currency"`
}

func New(amountMinor int64, currency string) (Money, error) {
	parsed, ok := ParseCurrency(currency)
	if !ok {
		return Money{}, ErrInvalidCurrency
	}
	return Money{AmountMinor: amountMinor, Currency: parsed}, nil
}

func (m Money) Add(other Money) (Money, error) {
	if err := requireSameCurrency(m, other); err != nil {
		return Money{}, err
	}
	return resultFromBig(new(big.Int).Add(big.NewInt(m.AmountMinor), big.NewInt(other.AmountMinor)), m.Currency)
}

func (m Money) Subtract(other Money) (Money, error) {
	if err := requireSameCurrency(m, other); err != nil {
		return Money{}, err
	}
	return resultFromBig(new(big.Int).Sub(big.NewInt(m.AmountMinor), big.NewInt(other.AmountMinor)), m.Currency)
}

func (m Money) MultiplyByScalar(scalar int64) (Money, error) { return m.MultiplyByFraction(scalar, 1) }

func (m Money) MultiplyByFraction(numerator, denominator int64) (Money, error) {
	product := new(big.Int).Mul(big.NewInt(m.AmountMinor), big.NewInt(numerator))
	rounded, err := roundHalfUp(product, big.NewInt(denominator))
	if err != nil {
		return Money{}, err
	}
	return resultFromBig(rounded, m.Currency)
}

func requireSameCurrency(left, right Money) error {
	if left.Currency != right.Currency {
		return ErrCurrencyMismatch
	}
	return nil
}

func resultFromBig(amount *big.Int, currency Currency) (Money, error) {
	if !amount.IsInt64() {
		return Money{}, ErrOverflow
	}
	return Money{AmountMinor: amount.Int64(), Currency: currency}, nil
}

func ParseCurrency(raw string) (Currency, bool) {
	currency := Currency(strings.ToUpper(strings.TrimSpace(raw)))
	if !isCurrencyCode(string(currency)) {
		return "", false
	}
	return currency, true
}

func isCurrencyCode(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}
func Zero(currency Currency) (Money, error) {
	parsed, ok := ParseCurrency(string(currency))
	if !ok {
		return Money{}, ErrInvalidCurrency
	}
	return Money{AmountMinor: 0, Currency: parsed}, nil
}

// Panicking versions of the above helpers
// only for tests and seeding scripts

func MustNew(amountMinor int64, currency Currency) Money {
	m, err := New(amountMinor, string(currency))
	if err != nil {
		panic(err)
	}
	return m
}
func (m Money) MustAdd(other Money) Money {
	res, err := m.Add(other)
	if err != nil {
		panic(err)
	}
	return res
}
func MustZero(currency Currency) Money {
	return MustNew(0, currency)
}
