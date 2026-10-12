// Package backfill is the historical backfill service (FR-DATA-003, ARCHITECTURE §5.5): jobs are
// split into Kite-sized windows, persisted in the candle DB, fetched through the shared limiter,
// resumed after restart, de-duplicated across consumers, and reported on the bus.
package backfill

import (
	"time"

	"tradeloom/internal/candles"
)

const day = 24 * time.Hour

// MaxWindow is Kite's maximum days per historical request per interval (A07 §A4).
var MaxWindow = map[candles.Interval]time.Duration{
	candles.Minute:   60 * day,
	candles.Minute3:  100 * day,
	candles.Minute5:  100 * day,
	candles.Minute10: 100 * day,
	candles.Minute15: 200 * day,
	candles.Minute30: 200 * day,
	candles.Minute60: 400 * day,
	candles.Day:      2000 * day,
}

// plan cuts each range into windows of at most MaxWindow[iv].
func plan(rs []candles.Range, iv candles.Interval) []candles.Range {
	var out []candles.Range
	for _, r := range rs {
		out = append(out, candles.Split(r, MaxWindow[iv])...)
	}
	return out
}

// halve splits a window in two at a whole-day boundary; ok=false when it is one day or less.
func halve(r candles.Range) ([]candles.Range, bool) {
	span := r.To.Sub(r.From)
	if span <= day {
		return nil, false
	}
	mid := r.From.Add((span / 2).Truncate(day))
	if !mid.After(r.From) {
		mid = r.From.Add(day)
	}
	return []candles.Range{{From: r.From, To: mid}, {From: mid, To: r.To}}, true
}
