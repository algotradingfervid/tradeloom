package builder

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/candles/sqlite"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ticker"
)

func ist(y int, m time.Month, d, h, min, s int) time.Time {
	return time.Date(y, m, d, h, min, s, 0, clock.IST)
}

type rig struct {
	b       *Builder
	clk     *clock.Fake
	cal     *calendar.Service
	st      *sqlite.Store
	closed  *bus.Sub[Event]
	forming *bus.Sub[Event]
}

func newRig(t *testing.T, now time.Time) *rig {
	t.Helper()
	cal, _ := calendar.NewDefault()
	fc := clock.NewFake(now)
	st, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "c.db"), sqlite.Options{Clock: fc, Calendar: cal})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	b := bus.New(nil)
	r := &rig{clk: fc, cal: cal, st: st,
		closed: bus.Subscribe(b, TopicClosed, bus.SubOpts[Event]{Buffer: 64}), forming: bus.Subscribe(b, TopicForming, bus.SubOpts[Event]{Buffer: 1024})}
	r.b = New(Options{Clock: fc, Calendar: cal, Store: st, Bus: b})
	return r
}

func tick(tok uint32, at time.Time, ltp float64, vol uint32) ticker.Tick {
	return ticker.Tick{Token: tok, LTP: ltp, Volume: vol, ExchangeTS: at, Received: at}
}

func next(t *testing.T, s *bus.Sub[Event]) Event {
	t.Helper()
	select {
	case e := <-s.C():
		return e
	case <-time.After(2 * time.Second):
		t.Fatal("no event")
	}
	return Event{}
}

// eventually polls read until it returns rows (or 2 s pass) and returns the last result.
func eventually(t *testing.T, read func() []candles.Candle) []candles.Candle {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		got := read()
		if len(got) > 0 || time.Now().After(deadline) {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitWritten waits until the writer goroutine has finished n batches.
func waitWritten(t *testing.T, b *Builder, n uint64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for b.written.Load() < n {
		if time.Now().After(deadline) {
			t.Fatalf("writer finished %d batches, want %d", b.written.Load(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// drainLast returns the last event delivered on s before it stays quiet for 100 ms.
func drainLast(s *bus.Sub[Event]) Event {
	var last Event
	for {
		select {
		case e := <-s.C():
			last = e
		case <-time.After(100 * time.Millisecond):
			return last
		}
	}
}

func TestFiveMinuteCandleFromTicksClosesAtBoundaryPlusGrace(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 9, 50, 0))
	r.b.Track(1, candles.Minute5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan ticker.Tick, 16)
	go r.b.Run(ctx, ticks)
	// Seed (Ruling 44): the subscription snapshot lands in the 09:50 bucket and sets the
	// volume baseline (0 here, so the 10:00 candle's volume stays 1320).
	ticks <- tick(1, ist(2026, 10, 12, 9, 52, 0), 99, 0)

	r.clk.Set(ist(2026, 10, 12, 10, 0, 0))
	for _, tk := range []ticker.Tick{
		tick(1, ist(2026, 10, 12, 10, 0, 0), 100, 1000), tick(1, ist(2026, 10, 12, 10, 1, 0), 103, 1100),
		tick(1, ist(2026, 10, 12, 10, 2, 0), 98, 1250), tick(1, ist(2026, 10, 12, 10, 4, 59), 101, 1300),
	} {
		ticks <- tk
	}
	if e := next(t, r.closed); !e.Partial || e.Candle.Start != ist(2026, 10, 12, 9, 50, 0) {
		t.Fatalf("seed candle %+v, want the partial 09:50 candle", e)
	}
	if !r.clk.WaitTimers(1, 2*time.Second) {
		t.Fatal("no close timer")
	}
	r.clk.Set(ist(2026, 10, 12, 10, 5, 0)) // boundary: still inside the 1 s grace
	ticks <- ticker.Tick{Token: 1, LTP: 102, Volume: 1320, ExchangeTS: ist(2026, 10, 12, 10, 4, 59), Received: r.clk.Now()}
	time.Sleep(20 * time.Millisecond)
	select {
	case e := <-r.closed.C():
		t.Fatalf("closed before grace: %+v", e)
	default:
	}
	r.clk.Set(ist(2026, 10, 12, 10, 5, 1))
	e := next(t, r.closed)
	want := candles.Candle{Start: ist(2026, 10, 12, 10, 0, 0), Open: 100, High: 103, Low: 98, Close: 102, Volume: 1320, Source: candles.SourceTick}
	if e.Candle != want || e.Partial || e.Interval != candles.Minute5 {
		t.Fatalf("closed %+v\nwant %+v", e.Candle, want)
	}
	got := eventually(t, func() []candles.Candle { // stored after publishing, by the writer goroutine
		got, _ := r.st.Read(ctx, 1, candles.Minute5, want.Start, want.Start.Add(5*time.Minute))
		return got
	})
	if len(got) != 1 || got[0].Complete || got[0].Source != candles.SourceTick || got[0].Close != 102 {
		t.Fatalf("stored %+v, want tick-built incomplete", got)
	}
	ticks <- tick(1, ist(2026, 10, 12, 10, 4, 59), 99, 1330) // after close: dropped
	ticks <- tick(1, ist(2026, 10, 12, 10, 5, 2), 104, 1400)
	time.Sleep(20 * time.Millisecond)
	if r.b.LateTicks() != 1 {
		t.Fatalf("late ticks = %d", r.b.LateTicks())
	}
	last := drainLast(r.forming) // Sub.C() is unbuffered: len() is always 0, so drain until quiet
	if last.Candle.Start != ist(2026, 10, 12, 10, 5, 0) || last.Candle.Volume != 80 || last.Candle.Open != 104 {
		t.Fatalf("next forming %+v (volume is the cumulative difference)", last.Candle)
	}
}

func TestPartialFirstCandleIsPublishedButNotStored(t *testing.T) { // Review Focus 4
	r := newRig(t, ist(2026, 10, 12, 10, 2, 30)) // chart opened mid-candle
	r.b.Track(2, candles.Minute5)
	r.b.OnTick(tick(2, ist(2026, 10, 12, 10, 2, 31), 50, 9000))
	if e := next(t, r.forming); !e.Partial {
		t.Fatal("first candle after tracking mid-bucket must be partial")
	}
	r.clk.Set(ist(2026, 10, 12, 10, 5, 1))
	r.b.CloseDue(r.clk.Now())
	if e := next(t, r.closed); !e.Partial {
		t.Fatal("closed event must keep Partial")
	}
	if got, _ := r.st.Read(context.Background(), 2, candles.Minute5, ist(2026, 10, 12, 10, 0, 0), ist(2026, 10, 12, 10, 5, 0)); len(got) != 0 {
		t.Fatalf("partial candle stored: %+v", got)
	}
	if n := len(r.b.writes); n != 0 {
		t.Fatalf("partial candle queued for the store (%d batches)", n)
	}
	r.b.OnTick(tick(2, ist(2026, 10, 12, 10, 5, 3), 51, 9100))
	if e := next(t, r.forming); e.Partial || e.Candle.Volume != 100 {
		t.Fatalf("second candle %+v", e)
	}
}

func TestSessionAnchorsHourlyAndSpecialSession(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 9, 0, 0))
	r.b.Track(3, candles.Minute60)
	r.b.OnTick(tick(3, ist(2026, 10, 12, 9, 10, 0), 10, 1)) // pre-open: ignored
	r.b.OnTick(tick(3, ist(2026, 10, 12, 15, 20, 0), 11, 5))
	if e := next(t, r.forming); e.Candle.Start != ist(2026, 10, 12, 15, 15, 0) {
		t.Fatalf("60m bucket start %v, want 15:15 (anchored at 09:15)", e.Candle.Start)
	}
	r.clk.Set(ist(2026, 10, 12, 15, 30, 1))
	if r.b.CloseDue(r.clk.Now()) != 1 {
		t.Fatal("last hourly candle must close at the 15:30 session end")
	}

	f, _ := calendar.ParseFile(strings.NewReader(calendar.Header+"\n2026-11-08,NSE,special,continuous,18:15,19:15\n"), "test")
	r.cal.MergeOverrides(f.Entries)
	r.b.Track(4, candles.Minute5)
	r.clk.Set(ist(2026, 11, 8, 18, 0, 0))
	r.b.OnTick(tick(4, ist(2026, 11, 8, 18, 16, 10), 20, 1))
	if e := next(t, r.forming); e.Candle.Start != ist(2026, 11, 8, 18, 15, 0) {
		t.Fatalf("special-session 5m start %v, want 18:15", e.Candle.Start)
	}
}

func TestTicksWithoutExchangeTimestampUseAppClock(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 11, 0, 0))
	r.b.Track(5, candles.Minute)
	r.b.OnTick(ticker.Tick{Token: 5, LTP: 7, Received: ist(2026, 10, 12, 11, 0, 30)})
	if e := next(t, r.forming); e.Candle.Start != ist(2026, 10, 12, 11, 0, 0) {
		t.Fatalf("start %v", e.Candle.Start)
	}
}

func TestRunReturnsWhenTickStreamIsClosed(t *testing.T) { // Manager.Raw closes its channel on cancel
	r := newRig(t, ist(2026, 10, 12, 10, 0, 0))
	ticks := make(chan ticker.Tick)
	done := make(chan struct{})
	go func() { r.b.Run(context.Background(), ticks); close(done) }()
	close(ticks)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run kept running on a closed tick stream")
	}
}

func TestOutOfOrderTicksNeverRewriteACandle(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 9, 0, 0))
	r.b.Track(6, candles.Minute5)
	r.b.OnTick(tick(6, ist(2026, 10, 12, 10, 5, 10), 100, 500))
	_ = next(t, r.forming)
	r.b.OnTick(tick(6, ist(2026, 10, 12, 10, 4, 58), 90, 450)) // previous bucket, arrived late
	if r.b.LateTicks() != 1 {
		t.Fatalf("tick for an earlier bucket must be dropped as late, late = %d", r.b.LateTicks())
	}
	r.b.OnTick(tick(6, ist(2026, 10, 12, 10, 6, 0), 104, 700))
	_ = next(t, r.forming)
	r.b.OnTick(tick(6, ist(2026, 10, 12, 10, 5, 30), 102, 600)) // same bucket, older than the last tick
	e := next(t, r.forming)
	want := candles.Candle{Start: ist(2026, 10, 12, 10, 5, 0), Open: 100, High: 104, Low: 100, Close: 104, Volume: 700, Source: candles.SourceTick}
	if e.Candle != want {
		t.Fatalf("got  %+v\nwant %+v (older tick must not move close or shrink volume)", e.Candle, want)
	}
}

func TestCumulativeVolumeRestartsOnANewDay(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 9, 0, 0))
	r.b.Track(7, candles.Minute5)
	r.b.OnTick(tick(7, ist(2026, 10, 12, 15, 29, 0), 10, 50000))
	_ = next(t, r.forming)
	r.clk.Set(ist(2026, 10, 13, 9, 0, 0))
	r.b.CloseDue(r.clk.Now())
	_ = next(t, r.closed)
	r.b.OnTick(tick(7, ist(2026, 10, 13, 9, 15, 2), 11, 300))
	r.b.OnTick(tick(7, ist(2026, 10, 13, 9, 16, 0), 12, 420))
	_ = next(t, r.forming)
	if e := next(t, r.forming); e.Candle.Volume != 420 || e.Partial {
		t.Fatalf("first candle of the next day %+v, want volume 420 (day volume restarts)", e)
	}
}

func TestZeroPriceTicksAreIgnored(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 9, 0, 0))
	r.b.Track(8, candles.Minute)
	r.b.OnTick(tick(8, ist(2026, 10, 12, 10, 0, 1), 0, 10))
	r.b.OnTick(tick(8, ist(2026, 10, 12, 10, 0, 2), 5, 20))
	if e := next(t, r.forming); e.Candle.Low != 5 || e.Candle.Open != 5 {
		t.Fatalf("zero-price tick used: %+v", e.Candle)
	}
}

func TestTrackedOnABoundaryWithoutBaselineIsPartial(t *testing.T) { // Ruling 44 (a)
	r := newRig(t, ist(2026, 10, 12, 10, 0, 0)) // tracked exactly at 10:00:00, after 09:15
	r.b.Track(9, candles.Minute5)
	r.b.OnTick(tick(9, ist(2026, 10, 12, 10, 0, 1), 50, 5000)) // 5000 is the whole day's volume so far
	if e := next(t, r.forming); !e.Partial {
		t.Fatalf("no volume baseline: the 10:00 candle must be partial, got %+v", e)
	}
	r.clk.Set(ist(2026, 10, 12, 10, 5, 1))
	if r.b.CloseDue(r.clk.Now()) != 1 {
		t.Fatal("candle not closed")
	}
	if e := next(t, r.closed); !e.Partial {
		t.Fatalf("closed %+v, want partial", e)
	}
	if n := len(r.b.writes); n != 0 {
		t.Fatalf("partial candle queued for the store (%d batches)", n)
	}
	r.b.OnTick(tick(9, ist(2026, 10, 12, 10, 5, 2), 51, 5100))
	if e := next(t, r.forming); e.Partial || e.Candle.Volume != 100 {
		t.Fatalf("next candle %+v, want exact (volume 100)", e)
	}
}

func TestSecondIntervalGetsTokenBaselineAtItsFirstBoundary(t *testing.T) { // Ruling 44 (a)
	r := newRig(t, ist(2026, 10, 12, 10, 0, 30))
	r.b.Track(10, candles.Minute)
	r.b.OnTick(tick(10, ist(2026, 10, 12, 10, 1, 0), 20, 1000))
	r.b.OnTick(tick(10, ist(2026, 10, 12, 10, 4, 50), 21, 1200))
	r.clk.Set(ist(2026, 10, 12, 10, 4, 55))
	r.b.Track(10, candles.Minute5) // already streaming; first 5m tick arrives after the boundary
	r.b.OnTick(tick(10, ist(2026, 10, 12, 10, 5, 3), 22, 1250))
	var got Event
	for got.Interval != candles.Minute5 {
		got = next(t, r.forming)
	}
	if got.Partial || got.Candle.Volume != 50 || got.Candle.Start != ist(2026, 10, 12, 10, 5, 0) {
		t.Fatalf("5m candle %+v, want exact with volume 50 (token baseline 1200)", got)
	}
}

func TestBuilderCloseLeavesBackfilledCompleteRowIntact(t *testing.T) {
	r := newRig(t, ist(2026, 10, 12, 9, 0, 0))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hist := candles.Candle{Start: ist(2026, 10, 12, 10, 0, 0), Open: 500, High: 510, Low: 495, Close: 505, Volume: 9999, Complete: true}
	if err := r.st.Upsert(ctx, 11, candles.Minute5, []candles.Candle{hist}, candles.SourceHistorical); err != nil {
		t.Fatal(err)
	}
	r.b.Track(11, candles.Minute5)
	ticks := make(chan ticker.Tick, 4)
	go r.b.Run(ctx, ticks)
	ticks <- tick(11, ist(2026, 10, 12, 10, 1, 0), 1, 10)
	ticks <- tick(11, ist(2026, 10, 12, 10, 2, 0), 2, 20)
	if !r.clk.WaitTimers(1, 2*time.Second) {
		t.Fatal("no close timer")
	}
	r.clk.Set(ist(2026, 10, 12, 10, 5, 1))
	if e := next(t, r.closed); e.Partial {
		t.Fatalf("closed %+v, want a storable candle", e)
	}
	waitWritten(t, r.b, 1)
	got, _ := r.st.Read(ctx, 11, candles.Minute5, hist.Start, hist.Start.Add(5*time.Minute))
	hist.Source = candles.SourceHistorical
	if len(got) != 1 || got[0] != hist {
		t.Fatalf("stored %+v, want the complete historical row %+v", got, hist)
	}
}

func TestDayCandleClosesAfterTheLastContinuousPhase(t *testing.T) { // Ruling 44 (d)
	r := newRig(t, ist(2026, 11, 8, 9, 0, 0))
	f, err := calendar.ParseFile(strings.NewReader(calendar.Header+
		"\n2026-11-08,NSE,special,continuous,10:00,11:00\n2026-11-08,NSE,special,continuous,18:15,19:15\n"), "test")
	if err != nil {
		t.Fatal(err)
	}
	r.cal.MergeOverrides(f.Entries)
	r.b.Track(12, candles.Day)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan ticker.Tick, 4)
	go r.b.Run(ctx, ticks)
	ticks <- tick(12, ist(2026, 11, 8, 10, 30, 0), 100, 100)
	ticks <- tick(12, ist(2026, 11, 8, 18, 30, 0), 110, 300)
	if !r.clk.WaitTimers(1, 2*time.Second) {
		t.Fatal("no close timer")
	}
	r.clk.Set(ist(2026, 11, 8, 11, 0, 1))
	select {
	case e := <-r.closed.C():
		t.Fatalf("day candle closed after the first phase: %+v", e)
	case <-time.After(100 * time.Millisecond):
	}
	r.clk.Set(ist(2026, 11, 8, 19, 15, 1))
	e := next(t, r.closed)
	want := candles.Candle{Start: ist(2026, 11, 8, 0, 0, 0), Open: 100, High: 110, Low: 100, Close: 110, Volume: 300, Source: candles.SourceTick}
	if e.Partial || e.Interval != candles.Day || e.Candle != want {
		t.Fatalf("day %+v\nwant %+v", e, want)
	}
	got := eventually(t, func() []candles.Candle {
		got, _ := r.st.Read(ctx, 12, candles.Day, want.Start, want.Start.Add(24*time.Hour))
		return got
	})
	if len(got) != 1 || got[0].Complete || got[0].Volume != 300 {
		t.Fatalf("stored %+v", got)
	}
}

func TestFullWriterQueueDropsAndCounts(t *testing.T) { // Ruling 44 (b): bounded queue
	r := newRig(t, ist(2026, 10, 12, 9, 0, 0)) // no Run: nothing drains the queue
	r.b.Track(13, candles.Minute)
	start := ist(2026, 10, 12, 10, 0, 0)
	for i := 0; i <= writeQueue; i++ {
		at := start.Add(time.Duration(i) * time.Minute)
		r.b.OnTick(tick(13, at.Add(time.Second), 10, uint32(100*(i+1))))
		r.b.CloseDue(at.Add(time.Minute + time.Second))
	}
	if len(r.b.writes) != writeQueue || r.b.DroppedWrites() != 1 {
		t.Fatalf("queued %d, dropped %d; want %d queued and 1 dropped", len(r.b.writes), r.b.DroppedWrites(), writeQueue)
	}
}
