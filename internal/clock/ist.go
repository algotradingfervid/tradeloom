package clock

import "time"

// MidnightIST returns 00:00 IST of the IST calendar day containing t.
func MidnightIST(t time.Time) time.Time {
	y, m, d := t.In(IST).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, IST)
}

// FormatIST renders t as DD-MM-YYYY HH:MM:SS in IST (NFR-11 date format).
func FormatIST(t time.Time) string { return t.In(IST).Format("02-01-2006 15:04:05") }

// FormatDateIST renders t as DD-MM-YYYY in IST.
func FormatDateIST(t time.Time) string { return t.In(IST).Format("02-01-2006") }

// NextBoundary returns the first instant anchor + k·step that is strictly after now
// (anchor itself when now is before anchor). step must be > 0.
func NextBoundary(now, anchor time.Time, step time.Duration) time.Time {
	if step <= 0 {
		panic("clock: NextBoundary step must be > 0")
	}
	if now.Before(anchor) {
		return anchor
	}
	k := now.Sub(anchor)/step + 1
	return anchor.Add(k * step)
}

// BucketStart returns anchor + floor((t−anchor)/step)·step. t must not be before anchor.
func BucketStart(t, anchor time.Time, step time.Duration) time.Time {
	if step <= 0 {
		panic("clock: BucketStart step must be > 0")
	}
	k := t.Sub(anchor) / step
	return anchor.Add(k * step)
}
