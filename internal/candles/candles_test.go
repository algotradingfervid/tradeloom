package candles

import (
	"strings"
	"testing"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
)

func ist(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, clock.IST)
}

func TestIntervals(t *testing.T) {
	for _, s := range []string{"minute", "3minute", "60minute", "day", "week", "month", "120minute", "45minute"} {
		if _, err := ParseInterval(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	for _, s := range []string{"hour", "0minute", "2day"} {
		if _, err := ParseInterval(s); err == nil {
			t.Errorf("%s accepted", s)
		}
	}
	if Minute15.Table() != "candles_15m" || Week.IsNative() || Interval("120minute").Base() != Minute60 ||
		Interval("45minute").Base() != Minute15 || Month.Base() != Day || Minute.Minutes() != 1 {
		t.Fatal("interval metadata wrong")
	}
	if ToPaise(2450.05) != 245005 || FromPaise(245005) != 2450.05 {
		t.Fatal("paise conversion")
	}
}

func TestRangesMergeSubtractSplit(t *testing.T) {
	r := func(a, b int) Range { return Range{ist(2026, 1, a, 0, 0), ist(2026, 1, b, 0, 0)} }
	m := Merge([]Range{r(5, 7), r(1, 3), r(3, 4), r(6, 9)})
	if len(m) != 2 || m[0] != r(1, 4) || m[1] != r(5, 9) {
		t.Fatalf("merge = %v", m)
	}
	miss := Subtract(r(1, 20), []Range{r(3, 5), r(8, 10), r(9, 12)})
	if len(miss) != 3 || miss[0] != r(1, 3) || miss[1] != r(5, 8) || miss[2] != r(12, 20) {
		t.Fatalf("subtract = %v", miss)
	}
	if got := Subtract(r(3, 4), []Range{r(1, 10)}); len(got) != 0 {
		t.Fatalf("fully covered = %v", got)
	}
	sp := Split(r(1, 11), 4*24*time.Hour)
	if len(sp) != 3 || sp[2] != r(9, 11) {
		t.Fatalf("split = %v", sp)
	}
}

func day(d int, o, h, l, c float64, v int64) Candle {
	return Candle{Start: ist(2026, 10, d, 0, 0), Open: o, High: h, Low: l, Close: c, Volume: v, Complete: true}
}

func TestWeeklyGolden(t *testing.T) { // FR-DATA-002 AC5 part 1
	week := []Candle{
		day(5, 100, 110, 95, 105, 1000), // Mon
		day(6, 105, 120, 101, 118, 2000),
		day(7, 118, 119, 90, 92, 1500),
		day(8, 92, 99, 91, 98, 1200),
		day(9, 98, 104, 97, 103, 800), // Fri
	}
	got := ResampleWeekly(week)
	want := Candle{Start: ist(2026, 10, 5, 0, 0), Open: 100, High: 120, Low: 90, Close: 103, Volume: 6500, Complete: true}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("week = %+v, want %+v", got, want)
	}
	m := ResampleMonthly(append(week, day(12, 103, 106, 100, 101, 10)))
	if len(m) != 1 || m[0].Start != ist(2026, 10, 1, 0, 0) || m[0].Close != 101 {
		t.Fatalf("month = %+v", m)
	}
}

func TestMuhuratCandlesBelongToTheirDate(t *testing.T) { // FR-DATA-002 AC5 part 2
	cal, _ := calendar.NewDefault()
	f, _ := calendar.ParseFile(strings.NewReader(calendar.Header+"\n2026-11-08,NSE,special,continuous,18:15,19:15\n"), "test")
	cal.MergeOverrides(f.Entries)
	var mins []Candle
	for i := 0; i < 60; i++ {
		p := 100 + float64(i)
		mins = append(mins, Candle{Start: ist(2026, 11, 8, 18, 15).Add(time.Duration(i) * time.Minute), Open: p, High: p + 1, Low: p - 1, Close: p + 0.5, Volume: 10, Complete: true})
	}
	d := DailyFromIntraday(mins)
	if len(d) != 1 || d[0].Start != ist(2026, 11, 8, 0, 0) || d[0].Volume != 600 || d[0].Open != 100 || d[0].Close != 159.5 {
		t.Fatalf("muhurat day = %+v", d)
	}
	five := ResampleIntraday(mins, 5, func(dt calendar.Date) (time.Time, time.Time, bool) { return cal.Session(calendar.NSE, dt) })
	if len(five) != 12 || five[0].Start.Format("15:04") != "18:15" || five[11].Start.Format("15:04") != "19:10" {
		t.Fatalf("5m starts: first %v last %v n=%d", five[0].Start, five[len(five)-1].Start, len(five))
	}
	w := ResampleWeekly(d)
	if w[0].Start != ist(2026, 11, 2, 0, 0) {
		t.Fatalf("Sunday session week start = %v, want Monday 02 Nov", w[0].Start)
	}
}

func TestSixtyMinuteBucketsAnchorAt0915AndCutAtClose(t *testing.T) {
	cal, _ := calendar.NewDefault()
	var mins []Candle
	for tm := ist(2026, 10, 12, 9, 15); tm.Before(ist(2026, 10, 12, 15, 30)); tm = tm.Add(time.Minute) {
		mins = append(mins, Candle{Start: tm, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, Complete: true})
	}
	h := ResampleIntraday(mins, 60, func(d calendar.Date) (time.Time, time.Time, bool) { return cal.Session(calendar.NSE, d) })
	if len(h) != 7 || h[0].Start.Format("15:04") != "09:15" || h[6].Start.Format("15:04") != "15:15" || h[6].Volume != 15 {
		t.Fatalf("60m = n%d first %s last %s vol %d", len(h), h[0].Start.Format("15:04"), h[6].Start.Format("15:04"), h[6].Volume)
	}
}

func TestBucketStartGuardAndIST(t *testing.T) {
	anchor := ist(2026, 10, 12, 9, 15)
	if got := bucketStart(anchor.Add(-7*time.Minute), anchor, 5*time.Minute); !got.Equal(anchor) {
		t.Fatalf("before anchor = %v, want anchor (not the next bucket)", got)
	}
	if got := bucketStart(anchor.Add(7*time.Minute).UTC(), anchor.UTC(), 5*time.Minute); got.Location() != clock.IST || !got.Equal(anchor.Add(5*time.Minute)) {
		t.Fatalf("UTC inputs = %v", got)
	}
	cal, _ := calendar.NewDefault()
	sess := func(d calendar.Date) (time.Time, time.Time, bool) { return cal.Session(calendar.NSE, d) }
	cs := []Candle{
		{Start: ist(2026, 10, 12, 9, 10), Open: 9, High: 9, Low: 9, Close: 9, Volume: 100, Complete: true}, // pre-open: dropped
		{Start: ist(2026, 10, 12, 9, 15).UTC(), Open: 1, High: 2, Low: 1, Close: 2, Volume: 1, Complete: true},
	}
	got := ResampleIntraday(cs, 5, sess)
	if len(got) != 1 || got[0].Volume != 1 || got[0].Start.Location() != clock.IST || got[0].Start.Format("15:04") != "09:15" {
		t.Fatalf("got %+v", got)
	}
}

func TestSplitAndResampleDegenerateArgs(t *testing.T) {
	r := Range{ist(2026, 1, 1, 0, 0), ist(2026, 1, 5, 0, 0)}
	for _, max := range []time.Duration{0, -time.Hour} {
		if got := Split(r, max); len(got) != 1 || got[0] != r {
			t.Fatalf("Split max=%v = %v", max, got)
		}
	}
	if got := Split(Range{r.To, r.From}, 0); got != nil {
		t.Fatalf("empty = %v", got)
	}
	cs := []Candle{{Start: ist(2026, 10, 12, 9, 15), Volume: 1, Complete: true}}
	for _, m := range []int{0, -5} {
		if got := ResampleIntraday(cs, m, func(calendar.Date) (time.Time, time.Time, bool) { return time.Time{}, time.Time{}, false }); got != nil {
			t.Fatalf("targetMin=%d = %v", m, got)
		}
	}
}
