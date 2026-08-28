package service

import (
	"context"
	"strings"
	"testing"

	"github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/shared/money"
)

func TestCreateRejectsInvalidBillingConfigurationBeforeRepositoryAccess(t *testing.T) {
	t.Parallel()

	svc := New(nil)
	cases := []struct {
		name  string
		input model.CreateInput
		want  error
	}{
		{"currency", model.CreateInput{Currency: "US", Timezone: "UTC"}, money.ErrInvalidCurrency},
		{"timezone", model.CreateInput{Currency: "USD", Timezone: "Mars/Olympus"}, nil},
		{"net terms", model.CreateInput{Currency: "USD", Timezone: "UTC", NetTerms: -1}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tc.input)
			if tc.want != nil {
				if err != tc.want {
					t.Fatalf("Create error = %v, want %v", err, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatal("Create error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tc.name) && tc.name != "net terms" {
				t.Fatalf("Create error = %v, want %s validation error", err, tc.name)
			}
		})
	}
}
