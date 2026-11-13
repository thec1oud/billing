// Package money provides integer-only monetary arithmetic for billing.
package money

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

var (
	ErrInvalidCurrency  = errors.New("currency must be a three-letter ISO 4217 code")
	ErrCurrencyMismatch = errors.New("cannot operate on money with different currencies")
	ErrDivisionByZero   = errors.New("money multiplier denominator must not be zero")
	ErrOverflow         = errors.New("money amount exceeds int64 range")
)

// enums will later be populated
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

var currencyMinorUnits = map[Currency]int{
	"BIF": 0, "CLP": 0, "DJF": 0, "GNF": 0, "ISK": 0,
	"JPY": 0, "KMF": 0, "KRW": 0, "PYG": 0, "RWF": 0,
	"UGX": 0, "UYI": 0, "VND": 0, "VUV": 0, "XAF": 0,
	"XOF": 0, "XPF": 0,

	"BHD": 3, "IQD": 3, "JOD": 3, "KWD": 3, "LYD": 3,
	"OMR": 3, "TND": 3,

	"CLF": 4, "UYW": 4,
}

var pow10 = [...]int64{
	1,     // 0
	10,    // 1
	100,   // 2
	1000,  // 3
	10000, // 4
}

// MinorUnits returns the number of minor units for a given currency.
func MinorUnits(currency Currency) int {
	if units, ok := currencyMinorUnits[currency]; ok {
		return units
	}

	// Assuming all unmapped ISO currencies default to 2
	return 2
}

// Format returns a formatted string representation of the money amount (e.g., "10.00 ETB", "-0.50 USD").
// It computes the exact value based on the ISO 4217 minor unit scale for the given currency.
func (m Money) Format() string {
	units := MinorUnits(m.Currency)
	if units == 0 {
		return fmt.Sprintf("%d %s", m.AmountMinor, m.Currency)
	}

	var divisor int64
	if units >= 0 && units < len(pow10) {
		divisor = pow10[units]
	} else {
		divisor = 100
	}

	sign := ""
	absAmount := m.AmountMinor
	if absAmount < 0 {
		sign = "-"
		absAmount = -absAmount
	}

	whole := absAmount / divisor
	fraction := absAmount % divisor

	return fmt.Sprintf(
		"%s%d.%0*d %s",
		sign,
		whole,
		units,
		fraction,
		m.Currency,
	)
}

// UnmarshalJSON validates that the currency is a valid ISO 4217 code when deserializing from JSON.
func (m *Money) UnmarshalJSON(data []byte) error {
	type moneyAlias struct {
		AmountMinor int64  `json:"amount_minor"`
		Currency    string `json:"currency"`
	}
	var aux moneyAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	parsed, ok := ParseCurrency(aux.Currency)
	if !ok {
		return ErrInvalidCurrency
	}
	m.AmountMinor = aux.AmountMinor
	m.Currency = parsed
	return nil
}
