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
// anchor days use the target month's final day. It is UTC-only; local-time/DST
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
