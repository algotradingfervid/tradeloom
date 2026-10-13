package clock

import (
	"sort"
	"time"
)

// halfSecond corrects for truncation: Kite tick timestamps and HTTP Date headers carry whole
// seconds, so the true instant lies uniformly in [ts, ts+1s); its expected value is ts+0.5 s.
const halfSecond = 500 * time.Millisecond

type tickSample struct {
	at  time.Time // system arrival time
	off time.Duration
}

// estimator keeps offset samples. It is not safe for concurrent use; Real guards it.
type estimator struct {
	window time.Duration // tick samples kept (60 s)
	httpN  int           // HTTP samples kept (5)
	ticks  []tickSample
	http   []time.Duration
}

func (e *estimator) addTick(exchangeTS, arrival time.Time) {
	e.ticks = append(e.ticks, tickSample{at: arrival, off: exchangeTS.Add(halfSecond).Sub(arrival)})
	e.prune(arrival)
}

func (e *estimator) addHTTP(date, sent, received time.Time) {
	mid := sent.Add(received.Sub(sent) / 2)
	e.http = append(e.http, date.Add(halfSecond).Sub(mid))
	if len(e.http) > e.httpN {
		e.http = e.http[len(e.http)-e.httpN:]
	}
}

func (e *estimator) prune(now time.Time) {
	cut := now.Add(-e.window)
	i := 0
	for i < len(e.ticks) && e.ticks[i].at.Before(cut) {
		i++
	}
	e.ticks = e.ticks[i:]
}

// estimate prefers ticks seen in the last window, then the last HTTP Date samples.
func (e *estimator) estimate(now time.Time) (time.Duration, Source, int) {
	e.prune(now)
	if len(e.ticks) > 0 {
		v := make([]time.Duration, len(e.ticks))
		for i, s := range e.ticks {
			v[i] = s.off
		}
		return median(v), SourceTicks, len(v)
	}
	if len(e.http) > 0 {
		v := append([]time.Duration(nil), e.http...)
		return median(v), SourceHTTPDate, len(v)
	}
	return 0, SourceNone, 0
}

func median(v []time.Duration) time.Duration {
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}
