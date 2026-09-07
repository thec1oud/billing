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
	plans     []Plan
	durations []PlanDuration

	nextPlanID     int64
	nextDurationID int64

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

	if f.nextPlanID == 0 {
		f.nextPlanID = 1
	}

	plan.ID = f.nextPlanID
	f.nextPlanID++

	f.plans = append(f.plans, plan)

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

	if f.nextDurationID == 0 {
		f.nextDurationID = 1
	}

	duration.ID = f.nextDurationID
	f.nextDurationID++

	f.durations = append(f.durations, duration)

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

func (f *fakePlanRepository) GetByIDAndVersion(
	_ context.Context,
	id int64,
	version int,
) (Plan, error) {
	if f.err != nil {
		return Plan{}, f.err
	}

	for _, plan := range f.plans {
		if plan.ID == id && plan.Version == version {
			return plan, nil
		}
	}

	return Plan{}, ErrPlanNotFound
}

func (f *fakePlanRepository) LatestVersion(
	_ context.Context,
	code string,
) (int, error) {
	if f.err != nil {
		return 0, f.err
	}

	latest := 0

	for _, plan := range f.plans {
		if plan.PlanCode == code && plan.Version > latest {
			latest = plan.Version
		}
	}

	return latest, nil
}

func (f *fakePlanRepository) ListVersions(
	_ context.Context,
	code string,
) ([]Plan, error) {
	if f.err != nil {
		return nil, f.err
	}

	result := make([]Plan, 0)

	for _, plan := range f.plans {
		if plan.PlanCode == code {
			result = append(result, plan)
		}
	}

	return result, nil
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

func (f *fakePlanRepository) GetDurationByPlanAndDuration(
	_ context.Context,
	planID int64,
	duration time.Duration,
) (PlanDuration, error) {
	if f.err != nil {
		return PlanDuration{}, f.err
	}

	for _, item := range f.durations {
		if item.PlanID == planID && item.Duration == duration {
			return item, nil
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

func TestCreatePlan(t *testing.T) {
	repository := &fakePlanRepository{
		plans: []Plan{
			{
				ID:                    1,
				PlanCode:              "PRO",
				Version:               1,
				LegacyPricePolicyCode: LegacyPolicyKeepForever,
			},
			{
				ID:                    2,
				PlanCode:              "PRO",
				Version:               2,
				LegacyPricePolicyCode: LegacyPolicyKeepForever,
			},
		},
		nextPlanID: 3,
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
	require.Equal(t, int64(3), created.ID)

	require.Len(t, repository.plans, 3)
	require.Equal(t, 3, repository.plans[2].Version)
}

func TestCreatePlanSetsEffectiveFromWhenMissing(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	before := time.Now().UTC()

	created, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode: "BASIC",
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
	require.Equal(
		t,
		LegacyPolicyKeepForever,
		created.LegacyPricePolicyCode,
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
	require.Empty(t, repository.plans)
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

func TestCreatePlanAutomaticallyCreatesNextVersion(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	first, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
			EffectiveFrom:         time.Now().UTC(),
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, first.Version)

	second, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyMigrateOnRenewal,
			EffectiveFrom:         time.Now().UTC(),
		},
	)

	require.NoError(t, err)
	require.Equal(t, 2, second.Version)

	third, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyMigrateImmediately,
			EffectiveFrom:         time.Now().UTC(),
		},
	)

	require.NoError(t, err)
	require.Equal(t, 3, third.Version)

	require.Len(t, repository.plans, 3)
	require.Equal(t, 1, repository.plans[0].Version)
	require.Equal(t, 2, repository.plans[1].Version)
	require.Equal(t, 3, repository.plans[2].Version)
}

func TestCreatePlanIgnoresCallerProvidedVersion(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	first, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			Version:               99,
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
			EffectiveFrom:         time.Now().UTC(),
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, first.Version)

	second, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			Version:               1,
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
			EffectiveFrom:         time.Now().UTC(),
		},
	)

	require.NoError(t, err)
	require.Equal(t, 2, second.Version)
}

func TestCreatePlanKeepsPreviousVersionImmutable(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	firstEffectiveFrom := time.Date(
		2026,
		1,
		1,
		0,
		0,
		0,
		0,
		time.UTC,
	)

	first, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
			EffectiveFrom:         firstEffectiveFrom,
			Metadata:              []byte(`{"price":100}`),
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, first.Version)
	require.Equal(t, int64(1), first.ID)

	secondEffectiveFrom := time.Date(
		2026,
		2,
		1,
		0,
		0,
		0,
		0,
		time.UTC,
	)

	second, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyMigrateOnRenewal,
			EffectiveFrom:         secondEffectiveFrom,
			Metadata:              []byte(`{"price":200}`),
		},
	)

	require.NoError(t, err)
	require.Equal(t, 2, second.Version)
	require.Equal(t, int64(2), second.ID)

	versionOne, err := service.GetPlanVersion(
		context.Background(),
		"PRO",
		1,
	)

	require.NoError(t, err)
	require.Equal(t, int64(1), versionOne.ID)
	require.Equal(t, 1, versionOne.Version)
	require.Equal(
		t,
		LegacyPolicyKeepForever,
		versionOne.LegacyPricePolicyCode,
	)
	require.Equal(t, firstEffectiveFrom, versionOne.EffectiveFrom)
	require.JSONEq(
		t,
		`{"price":100}`,
		string(versionOne.Metadata),
	)

	versionTwo, err := service.GetPlanVersion(
		context.Background(),
		"PRO",
		2,
	)

	require.NoError(t, err)
	require.Equal(t, int64(2), versionTwo.ID)
	require.Equal(t, 2, versionTwo.Version)
	require.Equal(
		t,
		LegacyPolicyMigrateOnRenewal,
		versionTwo.LegacyPricePolicyCode,
	)
	require.Equal(t, secondEffectiveFrom, versionTwo.EffectiveFrom)
	require.JSONEq(
		t,
		`{"price":200}`,
		string(versionTwo.Metadata),
	)
}

func TestCreatePlanCreatesIndependentVersionsForDifferentPlanCodes(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	proFirst, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, proFirst.Version)

	basicFirst, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "BASIC",
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, basicFirst.Version)

	proSecond, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
		},
	)

	require.NoError(t, err)
	require.Equal(t, 2, proSecond.Version)

	basicSecond, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "BASIC",
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
		},
	)

	require.NoError(t, err)
	require.Equal(t, 2, basicSecond.Version)
}

func TestCreatePlanDuration(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	duration := PlanDuration{
		PlanID:   1,
		TariffID: 10,
		Duration: 30 * 24 * time.Hour,
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
	require.Equal(t, 30*24*time.Hour, created.Duration)
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
			Duration: 0,
		},
	)

	require.Error(t, err)
	require.Empty(t, repository.durations)
}

func TestCreatePlanDurationKeepsDurationsAttachedToTheirPlanVersion(t *testing.T) {
	repository := &fakePlanRepository{}

	service := NewService(repository)

	firstPlan, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
		},
	)

	require.NoError(t, err)

	firstDuration, err := service.CreatePlanDuration(
		context.Background(),
		nil,
		PlanDuration{
			PlanID:   firstPlan.ID,
			TariffID: 100,
			Duration: 30 * 24 * time.Hour,
		},
	)

	require.NoError(t, err)

	secondPlan, err := service.CreatePlan(
		context.Background(),
		nil,
		Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: LegacyPolicyMigrateOnRenewal,
		},
	)

	require.NoError(t, err)

	secondDuration, err := service.CreatePlanDuration(
		context.Background(),
		nil,
		PlanDuration{
			PlanID:   secondPlan.ID,
			TariffID: 200,
			Duration: 30 * 24 * time.Hour,
		},
	)

	require.NoError(t, err)

	require.NotEqual(t, firstPlan.ID, secondPlan.ID)
	require.NotEqual(t, firstDuration.ID, secondDuration.ID)

	require.Equal(t, firstPlan.ID, firstDuration.PlanID)
	require.Equal(t, int64(100), firstDuration.TariffID)

	require.Equal(t, secondPlan.ID, secondDuration.PlanID)
	require.Equal(t, int64(200), secondDuration.TariffID)

	originalDurations, err := service.ListDurations(
		context.Background(),
		firstPlan.ID,
	)

	require.NoError(t, err)
	require.Len(t, originalDurations, 1)
	require.Equal(t, int64(100), originalDurations[0].TariffID)

	newDurations, err := service.ListDurations(
		context.Background(),
		secondPlan.ID,
	)

	require.NoError(t, err)
	require.Len(t, newDurations, 1)
	require.Equal(t, int64(200), newDurations[0].TariffID)
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

func TestGetPlanByIDAndVersion(t *testing.T) {
	expected := Plan{
		ID:                    10,
		PlanCode:              "PRO",
		Version:               3,
		LegacyPricePolicyCode: LegacyPolicyKeepForever,
	}

	repository := &fakePlanRepository{
		plans: []Plan{expected},
	}

	service := NewService(repository)

	result, err := service.GetPlanByIDAndVersion(
		context.Background(),
		10,
		3,
	)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestGetPlanByIDAndVersionRejectsInvalidID(t *testing.T) {
	service := NewService(&fakePlanRepository{})

	_, err := service.GetPlanByIDAndVersion(
		context.Background(),
		0,
		1,
	)

	require.Error(t, err)
}

func TestGetPlanByIDAndVersionRejectsInvalidVersion(t *testing.T) {
	service := NewService(&fakePlanRepository{})

	_, err := service.GetPlanByIDAndVersion(
		context.Background(),
		1,
		0,
	)

	require.Error(t, err)
}

func TestLatestVersion(t *testing.T) {
	repository := &fakePlanRepository{
		plans: []Plan{
			{
				ID:       1,
				PlanCode: "PRO",
				Version:  1,
			},
			{
				ID:       2,
				PlanCode: "PRO",
				Version:  4,
			},
			{
				ID:       3,
				PlanCode: "BASIC",
				Version:  9,
			},
		},
	}

	service := NewService(repository)

	result, err := service.LatestVersion(
		context.Background(),
		"PRO",
	)

	require.NoError(t, err)
	require.Equal(t, 4, result)
}

func TestLatestVersionRejectsEmptyCode(t *testing.T) {
	service := NewService(&fakePlanRepository{})

	_, err := service.LatestVersion(
		context.Background(),
		"",
	)

	require.Error(t, err)
}

func TestListVersions(t *testing.T) {
	expected := []Plan{
		{
			ID:                    2,
			PlanCode:              "PRO",
			Version:               2,
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
		},
		{
			ID:                    1,
			PlanCode:              "PRO",
			Version:               1,
			LegacyPricePolicyCode: LegacyPolicyKeepForever,
		},
	}

	repository := &fakePlanRepository{
		plans: expected,
	}

	service := NewService(repository)

	result, err := service.ListVersions(
		context.Background(),
		"PRO",
	)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestGetDuration(t *testing.T) {
	expected := PlanDuration{
		ID:       1,
		PlanID:   10,
		TariffID: 20,
		Duration: 30 * 24 * time.Hour,
		IsActive: true,
	}

	repository := &fakePlanRepository{
		durations: []PlanDuration{expected},
	}

	service := NewService(repository)

	result, err := service.GetDuration(
		context.Background(),
		1,
	)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestGetDurationByPlanAndCode(t *testing.T) {
	expected := PlanDuration{
		ID:       1,
		PlanID:   10,
		TariffID: 20,
		Duration: 30 * 24 * time.Hour,
		IsActive: true,
	}

	repository := &fakePlanRepository{
		durations: []PlanDuration{expected},
	}

	service := NewService(repository)

	result, err := service.GetDurationByPlanAndCode(
		context.Background(),
		10,
		30*24*time.Hour,
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
				Duration: 30 * 24 * time.Hour,
			},
			{
				ID:       2,
				PlanID:   10,
				TariffID: 30,
				Duration: 12 * 30 * 24 * time.Hour,
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
