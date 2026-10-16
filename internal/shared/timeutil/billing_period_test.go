package timeutil

import (
	"errors"
	"testing"
	"time"
)

func TestMonthlyPeriodEnd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		start     time.Time
		anchorDay int
		wantEnd   time.Time
		wantErr   error
	}{
		// Acceptance criterion: 29th start date
		{
			name:      "start 29th non-leap year clamps to Feb 28",
			start:     time.Date(2025, time.January, 29, 10, 0, 0, 0, time.UTC),
			anchorDay: 29,
			wantEnd:   time.Date(2025, time.February, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			name:      "start 29th leap year preserves Feb 29",
			start:     time.Date(2024, time.January, 29, 10, 0, 0, 0, time.UTC),
			anchorDay: 29,
			wantEnd:   time.Date(2024, time.February, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			name:      "start 29th March rolls over to April 29",
			start:     time.Date(2025, time.March, 29, 12, 30, 0, 0, time.UTC),
			anchorDay: 29,
			wantEnd:   time.Date(2025, time.April, 29, 12, 30, 0, 0, time.UTC),
		},

		// Acceptance criterion: 30th start date
		{
			name:      "start 30th non-leap year clamps to Feb 28",
			start:     time.Date(2025, time.January, 30, 8, 15, 0, 0, time.UTC),
			anchorDay: 30,
			wantEnd:   time.Date(2025, time.February, 28, 8, 15, 0, 0, time.UTC),
		},
		{
			name:      "start 30th leap year clamps to Feb 29",
			start:     time.Date(2024, time.January, 30, 8, 15, 0, 0, time.UTC),
			anchorDay: 30,
			wantEnd:   time.Date(2024, time.February, 29, 8, 15, 0, 0, time.UTC),
		},
		{
			name:      "start 30th March to April 30",
			start:     time.Date(2025, time.March, 30, 9, 0, 0, 0, time.UTC),
			anchorDay: 30,
			wantEnd:   time.Date(2025, time.April, 30, 9, 0, 0, 0, time.UTC),
		},
		{
			name:      "start 30th May to June 30",
			start:     time.Date(2025, time.May, 30, 9, 0, 0, 0, time.UTC),
			anchorDay: 30,
			wantEnd:   time.Date(2025, time.June, 30, 9, 0, 0, 0, time.UTC),
		},

		// Acceptance criterion: 31st start date
		{
			name:      "start 31st Jan non-leap clamps to Feb 28",
			start:     time.Date(2025, time.January, 31, 14, 30, 0, 0, time.UTC),
			anchorDay: 31,
			wantEnd:   time.Date(2025, time.February, 28, 14, 30, 0, 0, time.UTC),
		},
		{
			name:      "start 31st Jan leap year clamps to Feb 29",
			start:     time.Date(2024, time.January, 31, 14, 30, 0, 0, time.UTC),
			anchorDay: 31,
			wantEnd:   time.Date(2024, time.February, 29, 14, 30, 0, 0, time.UTC),
		},
		{
			name:      "start 31st March clamps to April 30",
			start:     time.Date(2025, time.March, 31, 16, 45, 0, 0, time.UTC),
			anchorDay: 31,
			wantEnd:   time.Date(2025, time.April, 30, 16, 45, 0, 0, time.UTC),
		},
		{
			name:      "start 31st May clamps to June 30",
			start:     time.Date(2025, time.May, 31, 23, 59, 0, 0, time.UTC),
			anchorDay: 31,
			wantEnd:   time.Date(2025, time.June, 30, 23, 59, 0, 0, time.UTC),
		},
		{
			name:      "start 31st July preserves August 31",
			start:     time.Date(2025, time.July, 31, 0, 0, 0, 0, time.UTC),
			anchorDay: 31,
			wantEnd:   time.Date(2025, time.August, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "start 31st August clamps to September 30",
			start:     time.Date(2025, time.August, 31, 18, 0, 0, 0, time.UTC),
			anchorDay: 31,
			wantEnd:   time.Date(2025, time.September, 30, 18, 0, 0, 0, time.UTC),
		},
		{
			name:      "start 31st October clamps to November 30",
			start:     time.Date(2025, time.October, 31, 7, 0, 0, 0, time.UTC),
			anchorDay: 31,
			wantEnd:   time.Date(2025, time.November, 30, 7, 0, 0, 0, time.UTC),
		},

		// Acceptance criterion: February in leap years
		{
			name:      "start Feb 29 in leap year rolls over to March 29",
			start:     time.Date(2024, time.February, 29, 11, 0, 0, 0, time.UTC),
			anchorDay: 29,
			wantEnd:   time.Date(2024, time.March, 29, 11, 0, 0, 0, time.UTC),
		},
		{
			name:      "start Feb 28 with anchor 29 in leap year rolls to March 29",
			start:     time.Date(2024, time.February, 28, 11, 0, 0, 0, time.UTC),
			anchorDay: 29,
			wantEnd:   time.Date(2024, time.March, 29, 11, 0, 0, 0, time.UTC),
		},
		{
			name:      "start Feb 28 with anchor 31 in non-leap year restores March 31",
			start:     time.Date(2025, time.February, 28, 11, 0, 0, 0, time.UTC),
			anchorDay: 31,
			wantEnd:   time.Date(2025, time.March, 31, 11, 0, 0, 0, time.UTC),
		},

		// Acceptance criterion: December rollover
		{
			name:      "December 31 rollover to next year January 31",
			start:     time.Date(2024, time.December, 31, 23, 0, 0, 0, time.UTC),
			anchorDay: 31,
			wantEnd:   time.Date(2025, time.January, 31, 23, 0, 0, 0, time.UTC),
		},
		{
			name:      "December 15 rollover to next year January 15",
			start:     time.Date(2025, time.December, 15, 10, 30, 0, 0, time.UTC),
			anchorDay: 15,
			wantEnd:   time.Date(2026, time.January, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:      "December 30 rollover to next year January 30",
			start:     time.Date(2025, time.December, 30, 8, 0, 0, 0, time.UTC),
			anchorDay: 30,
			wantEnd:   time.Date(2026, time.January, 30, 8, 0, 0, 0, time.UTC),
		},

		// Local timezone input converted to UTC
		{
			name:      "non-UTC start time is normalized to UTC",
			start:     time.Date(2025, time.January, 31, 14, 30, 0, 0, time.FixedZone("EAT", 3*60*60)),
			anchorDay: 31,
			wantEnd:   time.Date(2025, time.February, 28, 11, 30, 0, 0, time.UTC),
		},

		// Error conditions
		{
			name:      "zero time returns error",
			start:     time.Time{},
			anchorDay: 15,
			wantErr:   ErrZeroTime,
		},
		{
			name:      "anchor day 0 returns error",
			start:     time.Date(2025, time.January, 15, 0, 0, 0, 0, time.UTC),
			anchorDay: 0,
			wantErr:   ErrInvalidAnchor,
		},
		{
			name:      "anchor day 32 returns error",
			start:     time.Date(2025, time.January, 15, 0, 0, 0, 0, time.UTC),
			anchorDay: 32,
			wantErr:   ErrInvalidAnchor,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MonthlyPeriodEnd(tc.start, tc.anchorDay)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !got.Equal(tc.wantEnd) || got.Location() != time.UTC {
				t.Fatalf("got %s (%s); want %s UTC", got, got.Location(), tc.wantEnd)
			}
		})
	}
}
