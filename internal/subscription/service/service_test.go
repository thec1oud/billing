package service

import (
	"context"
	"strings"
	"testing"

	accountrepo "github.com/thec1oud/billing/internal/account/repository"
	planrepo "github.com/thec1oud/billing/internal/plan/repository"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/subscription/model"
)

func TestCreateValidation(t *testing.T) {
	t.Parallel()

	actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem, ID: "test"}

	tests := []struct {
		name        string
		svc         *Service
		input       model.CreateInput
		errContains string
	}{
		{
			name:        "invalid account id and plan id",
			svc:         New(nil, nil, nil, nil, nil, nil),
			input:       model.CreateInput{AccountID: 0, PlanID: 0, PlanVersion: 1},
			errContains: "account id and plan id must be greater than zero",
		},
		{
			name:        "invalid plan version",
			svc:         New(nil, nil, nil, nil, nil, nil),
			input:       model.CreateInput{AccountID: 1, PlanID: 1, PlanVersion: 0},
			errContains: "plan version must be greater than zero",
		},
		{
			name:        "nil plan repo",
			svc:         New(nil, nil, nil, nil, nil, nil),
			input:       model.CreateInput{AccountID: 1, PlanID: 1, PlanVersion: 1},
			errContains: "plan repository is not configured",
		},
		{
			name:        "nil account repo",
			svc:         New(nil, nil, &mockPlanRepo{}, nil, nil, nil),
			input:       model.CreateInput{AccountID: 1, PlanID: 1, PlanVersion: 1},
			errContains: "account repository is not configured",
		},
		{
			name:        "nil subscription repo",
			svc:         New(nil, &accountrepo.Repository{}, &mockPlanRepo{}, nil, nil, nil),
			input:       model.CreateInput{AccountID: 1, PlanID: 1, PlanVersion: 1},
			errContains: "subscription repository is not configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.svc.Create(context.Background(), actor, tc.input)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("expected error containing %q, got %v", tc.errContains, err)
			}
		})
	}
}

type mockPlanRepo struct {
	planrepo.Repository
}
