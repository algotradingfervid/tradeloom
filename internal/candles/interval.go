// Package candles defines the candle model, the Store interface (the OQ-41 swap point,
// ARCHITECTURE §4.3), coverage ranges and pure resampling. The SQLite implementation is in
// candles/sqlite, backfill in candles/backfill, tick building in candles/builder.
package candles

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Interval uses Kite's interval names; "week", "month" and "<N>minute" (N not native) are
// resampled on read and never stored (FR-DATA-002).
type Interval string

const (
	Minute   Interval = "minute"
	Minute3  Interval = "3minute"
	Minute5  Interval = "5minute"
	Minute10 Interval = "10minute"
	Minute15 Interval = "15minute"
	Minute30 Interval = "30minute"
	Minute60 Interval = "60minute"
	Day      Interval = "day"
	Week     Interval = "week"
	Month    Interval = "month"
)

// Native lists the eight stored intervals, smallest first.
var Native = []Interval{Minute, Minute3, Minute5, Minute10, Minute15, Minute30, Minute60, Day}

var tables = map[Interval]string{
	Minute: "candles_1m", Minute3: "candles_3m", Minute5: "candles_5m", Minute10: "candles_10m",
	Minute15: "candles_15m", Minute30: "candles_30m", Minute60: "candles_60m", Day: "candles_1d",
}

// ParseInterval validates s.
func ParseInterval(s string) (Interval, error) {
	iv := Interval(s)
	if iv == Day || iv == Week || iv == Month || iv == Minute {
		return iv, nil
	}
	if n, ok := strings.CutSuffix(s, "minute"); ok {
		if m, err := strconv.Atoi(n); err == nil && m >= 1 && m <= 375 {
			if m == 1 {
				return Minute, nil
			}
			return iv, nil
		}
	}
	return "", fmt.Errorf("candles: unknown interval %q", s)
}

// IsNative reports whether iv is stored.
func (iv Interval) IsNative() bool { _, ok := tables[iv]; return ok }

// Table is the SQLite table of a native interval ("" otherwise).
func (iv Interval) Table() string { return tables[iv] }

// Minutes is the length of an intraday interval; 0 for day, week, month.
func (iv Interval) Minutes() int {
	if iv == Minute {
		return 1
	}
	if n, ok := strings.CutSuffix(string(iv), "minute"); ok {
		m, _ := strconv.Atoi(n)
		return m
	}
	return 0
}

// IsIntraday reports whether iv is a minute-based interval.
func (iv Interval) IsIntraday() bool { return iv.Minutes() > 0 }

// Duration is the nominal length (intraday only; 24 h for day; 0 for week/month).
func (iv Interval) Duration() time.Duration {
	if iv == Day {
		return 24 * time.Hour
	}
	return time.Duration(iv.Minutes()) * time.Minute
}

// Base returns the native interval a non-native interval is resampled from: the largest native
// intraday interval dividing it, or Day for week/month.
func (iv Interval) Base() Interval {
	if iv == Week || iv == Month {
		return Day
	}
	m := iv.Minutes()
	for i := len(Native) - 2; i >= 0; i-- {
		if n := Native[i].Minutes(); n > 0 && m%n == 0 {
			return Native[i]
		}
	}
	return Minute
}
