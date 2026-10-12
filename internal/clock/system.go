// Package clock is the only package allowed to read the wall clock (ARCHITECTURE §2.4 rule 8).
// P0-1 provides SystemNow; P0-2 adds the offset-corrected Clock interface, timers and the estimator.
package clock

import "time"

// SystemNow returns the operating-system wall clock. Packages receive it as a
// func() time.Time value instead of calling time.Now themselves.
func SystemNow() time.Time { return time.Now() }
