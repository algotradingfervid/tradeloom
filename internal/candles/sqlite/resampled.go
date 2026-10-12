package sqlite

import (
	"context"
	"fmt"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
)

// ReadResampled builds week/month from day candles and non-native minute intervals from their
// base interval, anchored at each session start. It returns every period whose start lies in
// [periodStart(from), to): the base read is widened to whole periods, so a range that starts or
// ends inside a period yields that period's full bar, never a partial one. The last period is
// marked incomplete while it has not ended on the app clock.
func (s *Store) ReadResampled(ctx context.Context, token uint32, target candles.Interval, from, to time.Time) ([]candles.Candle, error) {
	if target.IsNative() {
		return s.Read(ctx, token, target, from, to)
	}
	var (
		start    func(t time.Time) time.Time // start of the period containing t
		end      func(start time.Time) time.Time
		resample func([]candles.Candle) []candles.Candle
	)
	switch {
	case target == candles.Week:
		start = func(t time.Time) time.Time {
			m := clock.MidnightIST(t)
			return m.AddDate(0, 0, -((int(m.Weekday()) + 6) % 7))
		}
		end = func(st time.Time) time.Time { return st.AddDate(0, 0, 7) }
		resample = candles.ResampleWeekly
	case target == candles.Month:
		start = func(t time.Time) time.Time {
			y, m, _ := t.In(clock.IST).Date()
			return time.Date(y, m, 1, 0, 0, 0, 0, clock.IST)
		}
		end = func(st time.Time) time.Time { return st.AddDate(0, 1, 0) }
		resample = candles.ResampleMonthly
	case target.IsIntraday():
		step := target.Duration()
		start = func(t time.Time) time.Time {
			ss, se, ok := s.ses(token, calendar.DateOf(t))
			if !ok || t.Before(ss) || !t.Before(se) {
				return t // outside a session: no bucket contains t
			}
			return ss.Add(t.Sub(ss) / step * step).In(clock.IST)
		}
		end = func(st time.Time) time.Time {
			e := st.Add(step)
			if _, se, ok := s.ses(token, calendar.DateOf(st)); ok && e.After(se) {
				e = se
			}
			return e
		}
		resample = func(cs []candles.Candle) []candles.Candle {
			return candles.ResampleIntraday(cs, target.Minutes(), func(d calendar.Date) (time.Time, time.Time, bool) { return s.ses(token, d) })
		}
	default:
		return nil, fmt.Errorf("candles: cannot resample to %q", target)
	}
	lo := start(from)
	hi := to
	if st := start(to); !st.Equal(to) {
		hi = end(st) // to falls inside a period: read the rest of it
	}
	src, err := s.Read(ctx, token, target.Base(), lo, hi)
	if err != nil {
		return nil, err
	}
	var out []candles.Candle
	for _, c := range resample(src) {
		if !c.Start.Before(lo) && c.Start.Before(to) {
			out = append(out, c)
		}
	}
	if n := len(out); n > 0 && s.clk != nil && end(out[n-1].Start).After(s.clk.Now()) {
		out[n-1].Complete = false
	}
	return out, nil
}
