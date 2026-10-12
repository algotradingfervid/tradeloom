package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/clock"
)

type grant struct {
	p  Priority
	at time.Time
}

func waitQueued(t *testing.T, l *Limiter, c Class, p Priority, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if l.Snapshot().Classes[c].Queued[p] == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("never saw %d queued at %v (snapshot %+v)", n, p, l.Snapshot().Classes[c])
}

// FR-DATA-003 AC1: 10 backtest requests queued, then one interactive: the interactive request is
// served before the next backtest request and no 1-second window has more than 3 historical grants.
func TestInteractiveOvertakesQueuedBacktestsAndRateHolds(t *testing.T) {
	fc := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	l := New(fc, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Run(ctx)

	var mu sync.Mutex
	var grants []grant
	var wg sync.WaitGroup
	acquire := func(p Priority) {
		defer wg.Done()
		if err := l.Acquire(ctx, ClassHistorical, p); err != nil {
			t.Error(err)
			return
		}
		mu.Lock()
		grants = append(grants, grant{p, fc.Now()})
		mu.Unlock()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go acquire(Backtest)
	}
	waitQueued(t, l, ClassHistorical, Backtest, 7) // three granted at once, seven wait
	wg.Add(1)
	go acquire(Interactive)
	waitQueued(t, l, ClassHistorical, Interactive, 1)
	for i := 0; i < 10; i++ {
		fc.Advance(250 * time.Millisecond)
		time.Sleep(5 * time.Millisecond)
	}
	for l.Snapshot().Classes[ClassHistorical].Queued[Backtest] > 0 {
		fc.Advance(250 * time.Millisecond)
		time.Sleep(5 * time.Millisecond)
	}
	wg.Wait()

	if len(grants) != 11 {
		t.Fatalf("grants = %d", len(grants))
	}
	// Goroutines append in scheduling order, so compare grant instants: exactly the initial
	// burst of three backtests may be granted strictly before the interactive request.
	var interAt time.Time
	for _, g := range grants {
		if g.p == Interactive {
			interAt = g.at
		}
	}
	before := 0
	for _, g := range grants {
		if g.p == Backtest && g.at.Before(interAt) {
			before++
		}
	}
	if before != 3 {
		t.Fatalf("%d backtest grants before the interactive one, want 3 (the initial burst)", before)
	}
	for i := range grants {
		n := 0
		for _, g := range grants {
			if !g.at.Before(grants[i].at) && g.at.Before(grants[i].at.Add(time.Second)) {
				n++
			}
		}
		if n > 3 {
			t.Fatalf("1-second window from grant %d holds %d historical grants", i, n)
		}
	}
}

// FR-DATA-003 AC2: a 429 on historical delays the next historical request ≥ 1 s; quote and
// orders are unaffected; the 429 counter is visible.
func TestBackoffIsolatedToClass(t *testing.T) {
	fc := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	l := New(fc, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Run(ctx)

	l.Backoff(ClassHistorical)
	start := fc.Now()
	if err := l.Acquire(ctx, ClassQuote, Interactive); err != nil {
		t.Fatal(err)
	}
	if err := l.Acquire(ctx, ClassOrders, Interactive); err != nil {
		t.Fatal(err)
	}
	if !fc.Now().Equal(start) {
		t.Fatal("quote/orders waited on a historical back-off")
	}
	done := make(chan time.Time, 1)
	go func() {
		if err := l.Acquire(ctx, ClassHistorical, Interactive); err == nil {
			done <- fc.Now()
		}
	}()
	waitQueued(t, l, ClassHistorical, Interactive, 1)
	fc.Advance(999 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("historical granted before 1 s back-off elapsed")
	case <-time.After(20 * time.Millisecond):
	}
	fc.Advance(time.Millisecond)
	select {
	case at := <-done:
		if at.Sub(start) < time.Second {
			t.Fatalf("granted after %v", at.Sub(start))
		}
	case <-time.After(time.Second):
		t.Fatal("historical never granted after back-off")
	}
	if got := l.Snapshot().Classes[ClassHistorical].HTTP429; got != 1 {
		t.Fatalf("http429 = %d", got)
	}
}

// FR-DATA-003 AC4 (limiter side): PauseAll stops every class until Resume.
func TestPauseAllAndResume(t *testing.T) {
	fc := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	l := New(fc, Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Run(ctx)
	l.PauseAll("session expired")
	got := make(chan Class, 4)
	for c := Class(0); c < numClasses; c++ {
		c := c
		go func() {
			if l.Acquire(ctx, c, Interactive) == nil {
				got <- c
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	if len(got) != 0 {
		t.Fatal("a class granted while paused")
	}
	if u := l.Snapshot(); !u.Paused || u.PauseReason != "session expired" {
		t.Fatalf("snapshot %+v", u)
	}
	l.Resume()
	for i := 0; i < int(numClasses); i++ {
		select {
		case <-got:
		case <-time.After(time.Second):
			t.Fatal("class not resumed")
		}
	}
}

func TestCancelledWaiterLeavesQueue(t *testing.T) {
	fc := clock.NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, clock.IST))
	l := New(fc, Options{})
	l.PauseAll("test")
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- l.Acquire(ctx, ClassQuote, Backtest) }()
	waitQueued(t, l, ClassQuote, Backtest, 1)
	cancel()
	if err := <-errc; err != context.Canceled {
		t.Fatalf("err = %v", err)
	}
	if n := l.Snapshot().Classes[ClassQuote].Queued[Backtest]; n != 0 {
		t.Fatalf("queued after cancel = %d", n)
	}
}

func TestPriorityContext(t *testing.T) {
	if PriorityFrom(context.Background()) != Interactive {
		t.Fatal("default must be Interactive")
	}
	if PriorityFrom(WithPriority(context.Background(), Backtest)) != Backtest {
		t.Fatal("WithPriority lost")
	}
}
