package clock

import "time"

// Now is allowed: internal/clock owns the wall clock.
func Now() time.Time { return time.Now() }
