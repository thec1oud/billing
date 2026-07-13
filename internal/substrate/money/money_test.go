package money

import (
	"errors"
	"testing"
)

func TestMoneyOperationsUseMinorUnits(t *testing.T) {
	fee, _ := New(1_250, "usd")
	credit, _ := New(250, "USD")
	total, err := fee.Add(credit)
	if err != nil || total != (Money{AmountMinor: 1_500, Currency: "USD"}) {
		t.Fatalf("add = %+v, %v", total, err)
	}
	remaining, err := total.Subtract(credit)
	if err != nil || remaining != fee {
		t.Fatalf("subtract = %+v, %v", remaining, err)
	}
	doubled, err := fee.MultiplyByScalar(2)
	if err != nil || doubled != (Money{AmountMinor: 2_500, Currency: "USD"}) {
		t.Fatalf("multiply = %+v, %v", doubled, err)
	}
}

func TestMoneyRejectsCurrencyMismatch(t *testing.T) {
	usd, _ := New(100, "USD")
	eur, _ := New(100, "EUR")
	if _, err := usd.Add(eur); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected mismatch, got %v", err)
	}
}

func TestRoundHalfUp(t *testing.T) {
	amount, _ := New(105, "USD")
	rounded, err := amount.MultiplyByFraction(1, 2)
	if err != nil || rounded.AmountMinor != 53 {
		t.Fatalf("105 / 2 = %+v, %v", rounded, err)
	}
}
