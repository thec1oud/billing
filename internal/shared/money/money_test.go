package money

import (
	"encoding/json"
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

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		money    Money
		expected string
	}{
		// Standard 2 decimal places (USD, ETB, etc.)
		{
			name:     "zero dollars",
			money:    Money{AmountMinor: 0, Currency: "USD"},
			expected: "0.00 USD",
		},
		{
			name:     "positive small cents",
			money:    Money{AmountMinor: 50, Currency: "USD"},
			expected: "0.50 USD",
		},
		{
			name:     "negative small cents (-0.50)",
			money:    Money{AmountMinor: -50, Currency: "USD"},
			expected: "-0.50 USD",
		},
		{
			name:     "positive one dollar fifty",
			money:    Money{AmountMinor: 150, Currency: "USD"},
			expected: "1.50 USD",
		},
		{
			name:     "negative one dollar fifty (-1.50)",
			money:    Money{AmountMinor: -150, Currency: "USD"},
			expected: "-1.50 USD",
		},
		{
			name:     "ETB standard formatting",
			money:    Money{AmountMinor: 1250, Currency: "ETB"},
			expected: "12.50 ETB",
		},
		{
			name:     "ETB small negative",
			money:    Money{AmountMinor: -5, Currency: "ETB"},
			expected: "-0.05 ETB",
		},

		// 0 decimal places (JPY, KRW, etc.)
		{
			name:     "zero JPY",
			money:    Money{AmountMinor: 0, Currency: "JPY"},
			expected: "0 JPY",
		},
		{
			name:     "positive JPY",
			money:    Money{AmountMinor: 1250, Currency: "JPY"},
			expected: "1250 JPY",
		},
		{
			name:     "negative JPY",
			money:    Money{AmountMinor: -50, Currency: "JPY"},
			expected: "-50 JPY",
		},
		{
			name:     "KRW zero decimal",
			money:    Money{AmountMinor: 500, Currency: "KRW"},
			expected: "500 KRW",
		},

		// 3 decimal places (BHD, KWD, JOD, etc.)
		{
			name:     "zero BHD",
			money:    Money{AmountMinor: 0, Currency: "BHD"},
			expected: "0.000 BHD",
		},
		{
			name:     "positive small BHD",
			money:    Money{AmountMinor: 50, Currency: "BHD"},
			expected: "0.050 BHD",
		},
		{
			name:     "negative small BHD",
			money:    Money{AmountMinor: -50, Currency: "BHD"},
			expected: "-0.050 BHD",
		},
		{
			name:     "positive KWD",
			money:    Money{AmountMinor: 1250, Currency: "KWD"},
			expected: "1.250 KWD",
		},
		{
			name:     "negative KWD",
			money:    Money{AmountMinor: -1250, Currency: "KWD"},
			expected: "-1.250 KWD",
		},

		// 4 decimal places (CLF, UYW)
		{
			name:     "CLF four decimals",
			money:    Money{AmountMinor: 1234, Currency: "CLF"},
			expected: "0.1234 CLF",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.money.Format()
			if actual != tc.expected {
				t.Errorf("Format() = %q, want %q", actual, tc.expected)
			}
		})
	}
}

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		inputJSON string
		expected  Money
		wantErr   error
	}{
		{
			name:      "valid USD",
			inputJSON: `{"amount_minor": 100, "currency": "USD"}`,
			expected:  Money{AmountMinor: 100, Currency: "USD"},
			wantErr:   nil,
		},
		{
			name:      "valid lowercase currency normalized",
			inputJSON: `{"amount_minor": 250, "currency": "etb"}`,
			expected:  Money{AmountMinor: 250, Currency: "ETB"},
			wantErr:   nil,
		},
		{
			name:      "negative amount",
			inputJSON: `{"amount_minor": -50, "currency": "EUR"}`,
			expected:  Money{AmountMinor: -50, Currency: "EUR"},
			wantErr:   nil,
		},
		{
			name:      "invalid currency code length",
			inputJSON: `{"amount_minor": 100, "currency": "US"}`,
			wantErr:   ErrInvalidCurrency,
		},
		{
			name:      "invalid currency non-alpha",
			inputJSON: `{"amount_minor": 100, "currency": "U1D"}`,
			wantErr:   ErrInvalidCurrency,
		},
		{
			name:      "empty currency",
			inputJSON: `{"amount_minor": 100, "currency": ""}`,
			wantErr:   ErrInvalidCurrency,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var m Money
			err := json.Unmarshal([]byte(tc.inputJSON), &m)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("UnmarshalJSON() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalJSON() unexpected error = %v", err)
			}
			if m != tc.expected {
				t.Errorf("UnmarshalJSON() = %+v, want %+v", m, tc.expected)
			}
		})
	}
}

func TestMinorUnits(t *testing.T) {
	if units := MinorUnits("JPY"); units != 0 {
		t.Errorf("MinorUnits(JPY) = %d, want 0", units)
	}
	if units := MinorUnits("USD"); units != 2 {
		t.Errorf("MinorUnits(USD) = %d, want 2", units)
	}
	if units := MinorUnits("ETB"); units != 2 {
		t.Errorf("MinorUnits(ETB) = %d, want 2", units)
	}
	if units := MinorUnits("BHD"); units != 3 {
		t.Errorf("MinorUnits(BHD) = %d, want 3", units)
	}
	if units := MinorUnits("CLF"); units != 4 {
		t.Errorf("MinorUnits(CLF) = %d, want 4", units)
	}
}
