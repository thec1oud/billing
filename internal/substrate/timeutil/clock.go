// Package timeutil centralizes the UTC-only time rules used by billing.
package timeutil

import "time"

func NowUTC() time.Time { return time.Now().UTC() }

// ToUTC normalizes a time before it is stored or compared.
func ToUTC(value time.Time) time.Time { return value.UTC() }
