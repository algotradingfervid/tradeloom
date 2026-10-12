package clock

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
)

// manualSystem is a settable system clock for Real.
type manualSystem struct {
	mu sync.Mutex
	t  time.Time
}

func (m *manualSystem) now() time.Time  { m.mu.Lock(); defer m.mu.Unlock(); return m.t }
func (m *manualSystem) set(t time.Time) { m.mu.Lock(); m.t = t; m.mu.Unlock() }

// feedTicks simulates 60 s of ~2 ticks/s from an exchange whose true time is exch, received
// by a system clock that runs `skew` ahead, with 20 ms network latency and random sub-second
// phase. It returns the true exchange time at the end.
func feedTicks(r *Real, ms *manualSystem, exch time.Time, skew time.Duration, seed int64) time.Time {
	rng := rand.New(rand.NewSource(seed))
	for i := 0; i < 120; i++ {
		exch = exch.Add(time.Duration(400+rng.Intn(200)) * time.Millisecond)
		arrival := exch.Add(skew).Add(20 * time.Millisecond)
		ms.set(arrival)
		r.ObserveTick(exch.Truncate(time.Second), arrival)
	}
	return exch
}

func TestOffsetFromTicksWithSystemClock3sFast(t *testing.T) { // FR-DATA-004 AC1
	ms := &manualSystem{}
	r := New(Options{System: ms.now})
	exch := time.Date(2026, 10, 12, 10, 0, 0, 0, IST)
	exch = feedTicks(r, ms, exch, 3*time.Second, 1)

	off := r.Offset()
	if off.Source != SourceTicks {
		t.Fatalf("source = %s, want ticks", off.Source)
	}
	if d := off.Value + 3*time.Second; d > 500*time.Millisecond || d < -500*time.Millisecond {
		t.Fatalf("offset = %v, want -3s ± 0.5s", off.Value)
	}
	// App clock equals exchange time ± 0.5 s, so a 1-minute candle boundary scheduled with
	// At() fires within ± 0.5 s of the exchange boundary.
	if d := r.Now().Sub(exch.Add(20 * time.Millisecond)); d > 500*time.Millisecond || d < -500*time.Millisecond {
		t.Fatalf("app clock off exchange by %v", d)
	}
	if !off.Warn || off.Skewed {
		t.Fatalf("3 s offset: warn=%v skewed=%v, want warn only", off.Warn, off.Skewed)
	}
}

func TestOffsetFromHTTPDateWhenNoTicks(t *testing.T) { // FR-DATA-004 AC4
	ms := &manualSystem{t: time.Date(2026, 10, 12, 18, 0, 0, 0, IST)}
	r := New(Options{System: ms.now})
	for i := 0; i < 5; i++ {
		sent := ms.now()
		received := sent.Add(80 * time.Millisecond)
		server := sent.Add(40 * time.Millisecond).Add(-2 * time.Second) // system 2 s fast
		r.ObserveHTTPDate(server.Truncate(time.Second), sent, received)
		ms.set(received.Add(time.Duration(300+100*i) * time.Millisecond))
	}
	off := r.Offset()
	if off.Source != SourceHTTPDate || off.Samples != 5 {
		t.Fatalf("source=%s samples=%d, want http-date/5", off.Source, off.Samples)
	}
	if d := off.Value + 2*time.Second; d > 500*time.Millisecond || d < -500*time.Millisecond {
		t.Fatalf("offset = %v, want -2s ± 0.5s", off.Value)
	}
}

func TestTicksWinOverHTTPAndExpireAfterWindow(t *testing.T) {
	ms := &manualSystem{t: time.Date(2026, 10, 12, 10, 0, 0, 0, IST)}
	r := New(Options{System: ms.now})
	r.ObserveHTTPDate(ms.now().Add(-4*time.Second), ms.now(), ms.now())
	feedTicks(r, ms, ms.now(), 0, 2)
	if r.Offset().Source != SourceTicks {
		t.Fatalf("ticks should win, got %s", r.Offset().Source)
	}
	ms.set(ms.now().Add(61 * time.Second)) // market closes; tick samples age out
	r.ObserveHTTPDate(ms.now().Truncate(time.Second), ms.now(), ms.now())
	if r.Offset().Source != SourceHTTPDate {
		t.Fatalf("after 61 s without ticks source = %s, want http-date", r.Offset().Source)
	}
}

func TestSkewHysteresisAndAcknowledge(t *testing.T) { // FR-DATA-004 AC3 (clock side)
	b := bus.New(nil)
	sub := bus.Subscribe(b, TopicOffset, bus.SubOpts[Offset]{Buffer: 64})
	ms := &manualSystem{t: time.Date(2026, 10, 12, 10, 0, 0, 0, IST)}
	r := New(Options{System: ms.now, Bus: b})
	observe := func(off time.Duration) {
		for i := 0; i < 130; i++ { // replace the whole 60 s window
			ms.set(ms.now().Add(500 * time.Millisecond))
			r.ObserveTick(ms.now().Add(off).Add(-halfSecond), ms.now())
		}
	}
	observe(7 * time.Second)
	if !r.Offset().Skewed {
		t.Fatal("7 s offset not skewed")
	}
	select {
	case o := <-sub.C():
		_ = o
	default:
		t.Fatal("no clock.offset event published")
	}
	observe(3 * time.Second)
	if !r.Offset().Skewed {
		t.Fatal("3 s is above the 2 s clear threshold; must stay skewed")
	}
	observe(1500 * time.Millisecond)
	if r.Offset().Skewed {
		t.Fatal("1.5 s must clear skew")
	}
	observe(7 * time.Second)
	r.Acknowledge()
	if r.Offset().Skewed {
		t.Fatal("acknowledged skew still reported")
	}
}

func TestRealTimerFiresAndRearmsOnOffsetChange(t *testing.T) {
	r := New(Options{}) // real system clock
	start := time.Now()
	tm := r.At(r.Now().Add(300 * time.Millisecond))
	// The exchange turns out to be 200 ms ahead: the timer must fire ~200 ms earlier.
	for i := 0; i < 5; i++ {
		r.ObserveHTTPDate(time.Now().Add(200*time.Millisecond-halfSecond), time.Now(), time.Now())
	}
	select {
	case <-tm.C():
		if el := time.Since(start); el > 250*time.Millisecond {
			t.Fatalf("fired after %v; offset change was not applied", el)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timer never fired")
	}
	if tm.Stop() {
		t.Fatal("Stop after fire must return false")
	}
}

func TestSlowDriftRearmsAndPublishes(t *testing.T) {
	b := bus.New(nil)
	sub := bus.Subscribe(b, TopicOffset, bus.SubOpts[Offset]{Buffer: 256})
	ms := &manualSystem{t: time.Date(2026, 10, 12, 10, 0, 0, 0, IST)}
	r := New(Options{System: ms.now, Bus: b})
	tm := r.At(ms.now().Add(time.Hour))
	defer tm.Stop()
	obs := func(off time.Duration) {
		for i := 0; i < 130; i++ { // replace the whole window
			ms.set(ms.now().Add(500 * time.Millisecond))
			r.ObserveTick(ms.now().Add(off).Add(-halfSecond), ms.now())
		}
	}
	obs(0)
	time.Sleep(100 * time.Millisecond)
	for len(sub.C()) > 0 {
		<-sub.C()
	}
	r.mu.Lock()
	base := r.armedValue
	r.mu.Unlock()
	// Each step is below Rearm (100 ms) but together they exceed it.
	for _, off := range []time.Duration{40, 80, 120, 160} {
		obs(off * time.Millisecond)
	}
	r.mu.Lock()
	armed := r.armedValue
	r.mu.Unlock()
	if d := armed - base; d < 100*time.Millisecond {
		t.Fatalf("armedValue moved %v, want >100ms after cumulative drift", d)
	}
	select {
	case <-sub.C():
	case <-time.After(time.Second):
		t.Fatal("no event published for cumulative drift")
	}
}

func TestAcknowledgeBeforeSkewDoesNotSwallowLaterSkew(t *testing.T) {
	ms := &manualSystem{t: time.Date(2026, 10, 12, 10, 0, 0, 0, IST)}
	r := New(Options{System: ms.now})
	obs := func(off time.Duration) {
		for i := 0; i < 130; i++ {
			ms.set(ms.now().Add(500 * time.Millisecond))
			r.ObserveTick(ms.now().Add(off).Add(-halfSecond), ms.now())
		}
	}
	obs(3 * time.Second)
	r.Acknowledge() // not skewed: no-op
	obs(7 * time.Second)
	if !r.Offset().Skewed {
		t.Fatal("premature Acknowledge swallowed a later skew")
	}
}

func TestConcurrentObserversPublishFinalState(t *testing.T) {
	b := bus.New(nil)
	sub := bus.Subscribe(b, TopicOffset, bus.SubOpts[Offset]{Buffer: 4096})
	r := New(Options{Bus: b})
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				now := time.Now()
				r.ObserveTick(now.Add(time.Duration(g*300+i*20)*time.Millisecond).Truncate(time.Second), now)
			}
		}(g)
	}
	wg.Wait()
	var last Offset
	var prevAt time.Time
	n := 0
drain:
	for {
		select {
		case last = <-sub.C():
			// UpdatedAt - Value is the system read time; it must never go backwards.
			if sysAt := last.UpdatedAt.Add(-last.Value); sysAt.Before(prevAt) {
				t.Fatalf("events out of order: system time %v < %v", sysAt, prevAt)
			} else {
				prevAt = sysAt
			}
			n++
		case <-time.After(200 * time.Millisecond):
			break drain
		}
	}
	if n == 0 {
		t.Fatal("no events")
	}
	// The last event must reflect the last significant state; state may differ only by
	// sub-Rearm drift after it.
	if fin := r.Offset(); last.Source != fin.Source || last.Warn != fin.Warn || last.Skewed != fin.Skewed ||
		abs(last.Value-fin.Value) > 100*time.Millisecond {
		t.Fatalf("last event %+v differs from final state %+v", last, fin)
	}
}

func TestObserveHTTPDateRejectsBadInput(t *testing.T) {
	ms := &manualSystem{t: time.Date(2026, 10, 12, 10, 0, 0, 0, IST)}
	r := New(Options{System: ms.now})
	r.ObserveHTTPDate(time.Time{}, ms.now(), ms.now())
	r.ObserveHTTPDate(ms.now(), ms.now().Add(time.Second), ms.now())
	if r.Offset().Samples != 0 {
		t.Fatal("invalid HTTP date samples were accepted")
	}
}
