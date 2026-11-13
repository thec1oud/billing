package timeutil

import (
	"errors"
	"time"
)

var (
	ErrInvalidAnchor = errors.New("billing cycle anchor must be between 1 and 31")
	ErrZeroTime      = errors.New("billing period start is required")
)

// MonthlyPeriodEnd returns the next monthly anchor after periodStart. Missing
// anchor days use the target month's final day (clamping days 29, 30, and 31
// when the target month has fewer days).
//
// Calculation is strictly UTC-normalized: local-time/DST adjustments are
// omitted so that billing periods remain predictable, monotonic, and immune
// to daylight saving shifts (such as 23-hour or 25-hour day anomalies).
// Account timezones are preserved on customer records for localized invoice
// presentation, reporting, and customer communications, while internal period
// boundaries and recurrence calculations operate exclusively in UTC.
func MonthlyPeriodEnd(periodStart time.Time, anchorDay int) (time.Time, error) {
	if periodStart.IsZero() {
		return time.Time{}, ErrZeroTime
	}
	if anchorDay < 1 || anchorDay > 31 {
		return time.Time{}, ErrInvalidAnchor
	}
	start := periodStart.UTC()
	year, month, _ := start.Date()
	targetMonth := month + 1
	if targetMonth > time.December {
		targetMonth = time.January
		year++
	}
	lastDay := time.Date(year, targetMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if anchorDay > lastDay {
		anchorDay = lastDay
	}
	return time.Date(
		year, targetMonth, anchorDay,
		start.Hour(), start.Minute(), start.Second(), start.Nanosecond(),
		time.UTC,
	), nil
}
