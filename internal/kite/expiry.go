// internal/kite/expiry.go
package kite

import (
	"time"

	"tradeloom/internal/clock"
)

// NextExpiry returns the first 06:00 IST strictly after t (A07 §A2: the token expires at 6 AM the next day;
// a login before 06:00 expires the same morning).
func NextExpiry(t time.Time) time.Time {
	in := t.In(clock.IST)
	six := time.Date(in.Year(), in.Month(), in.Day(), 6, 0, 0, 0, clock.IST)
	if !in.Before(six) {
		six = six.AddDate(0, 0, 1)
	}
	return six
}
