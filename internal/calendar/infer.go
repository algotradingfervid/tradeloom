package calendar

import (
	"context"
	"time"
)

// DailyCandleCheck reports whether a NIFTY 50 daily candle exists for d (fetching it first if
// needed). candles/backfill provides it; calendar only sees the function (rule 1).
type DailyCandleCheck func(ctx context.Context, d Date) (bool, error)

// Infer applies the FR-DATA-001 inference rule to d at time now: a weekday absent from the list
// with no NIFTY 50 daily candle after 16:00 is an inferred non-trading day. It returns the entry
// to persist when the day was marked, and ok=false when nothing changed.
func (s *Service) Infer(ctx context.Context, now time.Time, d Date, has DailyCandleCheck) (Entry, bool, error) {
	if d.IsWeekend() {
		return Entry{}, false, nil
	}
	if _, listed := s.lookup(NSE, d); listed {
		return Entry{}, false, nil
	}
	if now.Before(d.At(inferenceAfter)) {
		return Entry{}, false, nil
	}
	ok, err := has(ctx, d)
	if err != nil || ok {
		return Entry{}, false, err
	}
	e := Entry{Date: d, Exchange: NSE, Type: DayInferredClosed, Source: "inferred"}
	s.MergeOverrides([]Entry{e})
	return e, true, nil
}
