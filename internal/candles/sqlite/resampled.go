package sqlite

import (
	"context"
	"fmt"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
)

// ReadResampled builds week/month from day candles and non-native minute intervals from their
// base interval, anchored at each session start. The last period is marked incomplete while it
// has not ended on the app clock.
func (s *Store) ReadResampled(ctx context.Context, token uint32, target candles.Interval, from, to time.Time) ([]candles.Candle, error) {
	if target.IsNative() {
		return s.Read(ctx, token, target, from, to)
	}
	base := target.Base()
	src, err := s.Read(ctx, token, base, from, to)
	if err != nil {
		return nil, err
	}
	var out []candles.Candle
	var periodEnd func(c candles.Candle) time.Time
	switch {
	case target == candles.Week:
		out = candles.ResampleWeekly(src)
		periodEnd = func(c candles.Candle) time.Time { return c.Start.AddDate(0, 0, 7) }
	case target == candles.Month:
		out = candles.ResampleMonthly(src)
		periodEnd = func(c candles.Candle) time.Time { return c.Start.AddDate(0, 1, 0) }
	case target.IsIntraday():
		out = candles.ResampleIntraday(src, target.Minutes(), func(d calendar.Date) (time.Time, time.Time, bool) { return s.ses(token, d) })
		periodEnd = func(c candles.Candle) time.Time {
			_, end, _ := s.ses(token, calendar.DateOf(c.Start))
			e := c.Start.Add(target.Duration())
			if e.After(end) {
				e = end
			}
			return e
		}
	default:
		return nil, fmt.Errorf("candles: cannot resample to %q", target)
	}
	if n := len(out); n > 0 && s.clk != nil && periodEnd(out[n-1]).After(s.clk.Now()) {
		out[n-1].Complete = false
	}
	return out, nil
}
