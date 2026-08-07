package plan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stretchr/testify/require"
)

type fakePlanRepository struct {
	latestVersion int
	createdPlan   Plan

	createdDuration PlanDuration

	plans     []Plan
	durations []PlanDuration

	err error
}

func (f *fakePlanRepository) Create(
	_ context.Context,
	_ pgx.Tx,
	plan Plan,
) (Plan, error) {
	if f.err != nil {
		return Plan{}, f.err
	}

	f.createdPlan = plan

	if plan.ID == 0 {
		plan.ID = 1
	}

	return plan, nil
}

func (f *fakePlanRepository) CreateDuration(
	_ context.Context,
	_ pgx.Tx,
	duration PlanDuration,
) (PlanDuration, error) {
	if f.err != nil {
		return PlanDuration{}, f.err
	}

	f.createdDuration = duration

	if duration.ID == 0 {
		duration.ID = 1
	}

	return duration, nil
}

func (f *fakePlanRepository) GetByCodeAndVersion(
	_ context.Context,
	code string,
	version int,
) (Plan, error) {
	if f.err != nil {
		return Plan{}, f.err
	}

	for _, plan := range f.plans {
		if plan.PlanCode == code && plan.Version == version {
			return plan, nil
		}
	}

	return Plan{}, ErrPlanNotFound
}

func (f *fakePlanRepository) LatestVersion(
	_ context.Context,
	_ string,
) (int, error) {
	if f.err != nil {
		return 0, f.err
	}

	return f.latestVersion, nil
}

func (f *fakePlanRepository) ListVersions(
	_ context.Context,
	_ string,
) ([]Plan, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.plans, nil
}

func (f *fakePlanRepository) GetDuration(
	_ context.Context,
	id int64,
) (PlanDuration, error) {
	if f.err != nil {
		return PlanDuration{}, f.err
	}

	for _, duration := range f.durations {
		if duration.ID == id {
			return duration, nil
		}
	}

	return PlanDuration{}, ErrPlanDurationNotFound
}

func (f *fakePlanRepository) GetDurationByPlanAndCode(
	_ context.Context,
	planID int64,
	code PlanDurationCode,
) (PlanDuration, error) {
	if f.err != nil {
		return PlanDuration{}, f.err
	}

	for _, duration := range f.durations {
		if duration.PlanID == planID && duration.Duration == code {
			return duration, nil
		}
	}

	return PlanDuration{}, ErrPlanDurationNotFound
}

func (f *fakePlanRepository) ListDurations(
	_ context.Context,
	planID int64,
) ([]PlanDuration, error) {
	if f.err != nil {
		return nil, f.err
	}

	result := make([]PlanDuration, 0)

	for _, duration := range f.durations {
		if duration.PlanID == planID {
			result = append(result, duration)
		}
	}

	return result, nil
}

func (f *fakePlanRepository) UpdateDurationTariff(
	_ context.Context,
	_ pgx.Tx,
	durationID int64,
	tariffID int64,
	isActive bool,
) (PlanDuration, error) {
	if f.err != nil {
		return PlanDuration{}, f.err
	}

	for _, duration := range f.durations {
		if duration.ID == durationID {
			duration.TariffID = tariffID
			duration.IsActive = isActive
			return duration, nil
		}
	}

	return PlanDuration{}, ErrPlanDurationNotFound
}

func TestCreatePlan(t *testing.T) {
	repository := &fakePlanRepository{
		latestVersion: 2,
	}

	service := NewService(repository)

	ctx := context.Background()

	plan := Plan{
		PlanCode:              "PRO",
		LegacyPricePolicyCode: LegacyPolicyKeepForever,
		EffectiveFrom:         time.Now().UTC(),
	}

	created, err := service.CreatePlan(ctx, nil, plan)

	require.NoError(t, err)
	require.Equal(t, 3, created.Version)
	require.Equal(t, "PRO", created.PlanCode)
	require.Equal(t, LegacyPolicyKeepForever, created.LegacyPricePolicyCode)
	require.NotZero(t, created.ID)

	require.Equal(t, 3, repository.createdPlan.Version)
}

func TestCreatePlanSetsEffectiveFromWhenMissing(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	before := time.Now().UTC()

	created, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "BASIC",
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
		},
	)

	after := time.Now().UTC()

	require.NoError(t, err)
	require.False(t, created.EffectiveFrom.IsZero())
	require.True(
		t,
		!created.EffectiveFrom.Before(before) &&
			!created.EffectiveFrom.After(after),
	)
}

func TestCreatePlanRejectsInvalidPlan(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	_, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode: "",
		},
	)

	require.Error(t, err)
}

func TestCreatePlanRepositoryError(t *testing.T) {
	repositoryError := errors.New("database error")

	repository := &fakePlanRepository{
		err: repositoryError,
	}

	service := NewService(repository)

	_, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
			EffectiveFrom:         time.Now().UTC(),
		},
	)

	require.Error(t, err)
	require.ErrorIs(t, err, repositoryError)
}

func TestCreatePlanDuration(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	duration := PlanDuration{
		PlanID:   1,
		TariffID: 10,
		Duration: PlanDurationCode("MONTHLY"),
	}

	created, err := service.CreatePlanDuration(
		context.Background(),
		nil,
		duration,
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), created.ID)
	require.Equal(t, int64(1), created.PlanID)
	require.Equal(t, int64(10), created.TariffID)
	require.Equal(t, PlanDurationCode("MONTHLY"), created.Duration)
	require.True(t, created.IsActive)
}

func TestCreatePlanDurationRejectsInvalidDuration(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	_, err := service.CreatePlanDuration(
		context.Background(),
		nil,
		PlanDuration{
			PlanID:   1,
			TariffID: 10,
			Duration: "INVALID",
		},
	)

	require.Error(t, err)
}

func TestGetPlanVersion(t *testing.T) {
	expected := Plan{
		ID:                    1,
		PlanCode:              "PRO",
		Version:               2,
		LegacyPricePolicyCode: LegacyPolicyKeepForever,
	}

	repository := &fakePlanRepository{
		plans: []Plan{expected},
	}

	service := NewService(repository)

	result, err := service.GetPlanVersion(
		context.Background(),
		"PRO",
		2,
	)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestGetPlanVersionRejectsInvalidVersion(t *testing.T) {
	service := NewService(&fakePlanRepository{})

	_, err := service.GetPlanVersion(
		context.Background(),
		"PRO",
		0,
	)

	require.Error(t, err)
}

func TestGetDurationByPlanAndCode(t *testing.T) {
	expected := PlanDuration{
		ID:       1,
		PlanID:   10,
		TariffID: 20,
		Duration: PlanDurationCode("MONTHLY"),
		IsActive: true,
	}

	repository := &fakePlanRepository{
		durations: []PlanDuration{expected},
	}

	service := NewService(repository)

	result, err := service.GetDurationByPlanAndCode(
		context.Background(),
		10,
		PlanDurationCode("MONTHLY"),
	)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestListDurations(t *testing.T) {
	repository := &fakePlanRepository{
		durations: []PlanDuration{
			{
				ID:       1,
				PlanID:   10,
				TariffID: 20,
				Duration: PlanDurationCode("MONTHLY"),
			},
			{
				ID:       2,
				PlanID:   10,
				TariffID: 30,
				Duration: PlanDurationCode("YEARLY"),
			},
		},
	}

	service := NewService(repository)

	result, err := service.ListDurations(
		context.Background(),
		10,
	)

	require.NoError(t, err)
	require.Len(t, result, 2)
}

func TestUpdateDurationTariff(t *testing.T) {
	repository := &fakePlanRepository{
		durations: []PlanDuration{
			{
				ID:       1,
				PlanID:   10,
				TariffID: 20,
				Duration: PlanDurationCode("MONTHLY"),
				IsActive: true,
			},
		},
	}

	service := NewService(repository)

	result, err := service.UpdateDurationTariff(
		context.Background(),
		nil,
		1,
		99,
		false,
	)

	require.NoError(t, err)
	require.Equal(t, int64(99), result.TariffID)
	require.False(t, result.IsActive)
}
