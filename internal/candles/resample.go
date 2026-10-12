package candles

import (
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
)

// SessionFunc returns the continuous session of IST date d for one instrument's exchange.
type SessionFunc func(d calendar.Date) (start, end time.Time, ok bool)

type agg struct {
	c   Candle
	set bool
}

func (a *agg) add(c Candle) {
	if !a.set {
		a.c = c
		a.set = true
		return
	}
	if c.High > a.c.High {
		a.c.High = c.High
	}
	if c.Low < a.c.Low {
		a.c.Low = c.Low
	}
	a.c.Close = c.Close
	a.c.Volume += c.Volume
	if c.HasOI {
		a.c.OI, a.c.HasOI = c.OI, true
	}
	a.c.Complete = a.c.Complete && c.Complete
	if c.Source != a.c.Source {
		a.c.Source = SourceTick
	}
}

func (a *agg) flush(start time.Time, out []Candle) []Candle {
	if !a.set {
		return out
	}
	a.c.Start = start
	out = append(out, a.c)
	a.set = false
	return out
}

// ResampleIntraday aggregates ascending intraday candles into targetMin-minute buckets anchored
// at each day's session start (FR-DATA-002, FR-BT-034); the last bucket of a session is cut at the
// session end (e.g. 60-minute: 09:15, 10:15, …, 15:15–15:30). Candles outside the session drop.
func ResampleIntraday(cs []Candle, targetMin int, session SessionFunc) []Candle {
	step := time.Duration(targetMin) * time.Minute
	var out []Candle
	var a agg
	var bucket time.Time
	var day calendar.Date
	var start, end time.Time
	var ok bool
	for _, c := range cs {
		d := calendar.DateOf(c.Start)
		if d != day {
			day = d
			start, end, ok = session(d)
		}
		if !ok || c.Start.Before(start) || !c.Start.Before(end) {
			continue
		}
		b := bucketStart(c.Start, start, step)
		if a.set && !b.Equal(bucket) {
			out = a.flush(bucket, out)
		}
		bucket = b
		a.add(c)
	}
	return a.flush(bucket, out)
}

// DailyFromIntraday aggregates intraday candles per IST calendar date; a special session (e.g.
// an evening Muhurat session) therefore belongs to its own date's day candle (FR-DATA-002 AC5).
func DailyFromIntraday(cs []Candle) []Candle {
	return byPeriod(cs, func(t time.Time) time.Time { return clock.MidnightIST(t) })
}

// ResampleWeekly aggregates daily candles into Monday-starting weeks (Start = Monday 00:00 IST);
// a weekend special session joins the week of the preceding Monday.
func ResampleWeekly(cs []Candle) []Candle {
	return byPeriod(cs, func(t time.Time) time.Time {
		m := clock.MidnightIST(t)
		off := (int(m.Weekday()) + 6) % 7 // days since Monday
		return m.AddDate(0, 0, -off)
	})
}

// ResampleMonthly aggregates daily candles per calendar month (Start = 1st, 00:00 IST).
func ResampleMonthly(cs []Candle) []Candle {
	return byPeriod(cs, func(t time.Time) time.Time {
		y, m, _ := t.In(clock.IST).Date()
		return time.Date(y, m, 1, 0, 0, 0, 0, clock.IST)
	})
}

func byPeriod(cs []Candle, key func(time.Time) time.Time) []Candle {
	var out []Candle
	var a agg
	var cur time.Time
	for _, c := range cs {
		k := key(c.Start)
		if a.set && !k.Equal(cur) {
			out = a.flush(cur, out)
		}
		cur = k
		a.add(c)
	}
	return a.flush(cur, out)
}

// bucketStart wraps clock.BucketStart: clock truncates toward zero for t before anchor (returning the
// NEXT bucket) and returns times in the anchor's location, so t < anchor yields anchor and the
// result is always in IST.
func bucketStart(t, anchor time.Time, step time.Duration) time.Time {
	if t.Before(anchor) {
		return anchor.In(clock.IST)
	}
	return clock.BucketStart(t, anchor, step).In(clock.IST)
}
