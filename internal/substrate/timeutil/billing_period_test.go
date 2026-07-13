package timeutil

import (
	"testing"
	"time"
)

func TestMonthlyPeriodEndClampsJanuary31ToFebruary(t *testing.T) {
	start := time.Date(2025, time.January, 31, 14, 30, 0, 0, time.FixedZone("EAT", 3*60*60))
	end, err := MonthlyPeriodEnd(start, 31)
	want := time.Date(2025, time.February, 28, 11, 30, 0, 0, time.UTC)
	if err != nil || !end.Equal(want) || end.Location() != time.UTC {
		t.Fatalf("period end = %s (%s), %v; want %s UTC", end, end.Location(), err, want)
	}
}
