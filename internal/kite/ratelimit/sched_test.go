package ratelimit

import (
	"container/heap"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

func push(c *classState, p Priority, seq uint64) *waiter {
	w := &waiter{prio: p, seq: seq, ready: make(chan struct{})}
	heap.Push(&c.q, w)
	return w
}

func TestSlidingWindowNeverExceedsRate(t *testing.T) {
	c := newClassState(3, time.Second)
	for i := 0; i < 20; i++ {
		push(c, Backtest, uint64(i))
	}
	var grants []time.Time
	now := t0
	for len(grants) < 20 {
		w, wait := c.step(now, false)
		if w != nil {
			grants = append(grants, now)
			continue
		}
		now = now.Add(wait)
	}
	for i := range grants {
		n := 0
		for _, g := range grants {
			if !g.Before(grants[i]) && g.Before(grants[i].Add(time.Second)) {
				n++
			}
		}
		if n > 3 {
			t.Fatalf("window starting %v holds %d grants", grants[i].Sub(t0), n)
		}
	}
	if got := grants[19].Sub(t0); got != 6*time.Second {
		t.Fatalf("20 grants at 3/s should finish at +6s, finished at +%v", got)
	}
}

func TestPriorityThenFIFO(t *testing.T) {
	c := newClassState(3, time.Second)
	for i := 0; i < 10; i++ {
		push(c, Backtest, uint64(i))
	}
	push(c, Library, 100)
	inter := push(c, Interactive, 200)
	w, _ := c.step(t0, false)
	if w != inter {
		t.Fatal("interactive must be granted before queued backtests")
	}
	w, _ = c.step(t0, false)
	if w.prio != Backtest || w.seq != 0 {
		t.Fatalf("then oldest backtest, got %v seq %d", w.prio, w.seq)
	}
}

func TestBackoffDoublesToThirtySeconds(t *testing.T) {
	c := newClassState(3, time.Second)
	want := []time.Duration{1, 2, 4, 8, 16, 30, 30}
	for i, s := range want {
		if d := c.backoff(t0); d != s*time.Second {
			t.Fatalf("backoff %d = %v, want %v", i, d, s*time.Second)
		}
	}
	if c.http429 != 7 {
		t.Fatalf("http429 = %d", c.http429)
	}
}

func TestPausedGrantsNothing(t *testing.T) {
	c := newClassState(3, time.Second)
	push(c, Interactive, 1)
	if w, wait := c.step(t0, true); w != nil || wait != -1 {
		t.Fatal("paused class granted")
	}
}

func TestStaleSuccessAndLateBackoffNeverShortenHold(t *testing.T) {
	c := newClassState(3, time.Second)
	for i := 0; i < 4; i++ { // holds 1,2,4,8 s
		c.backoff(t0)
	}
	before := c.backoffUntil       // t0+8s
	c.success(t0.Add(time.Second)) // stale success while the hold is active
	if c.backoffN == 0 {
		t.Fatal("success inside an active hold reset the exponent")
	}
	c.backoff(t0.Add(time.Second)) // late 429: 16 s from +1s = +17s, extends
	if c.backoffUntil.Before(before) {
		t.Fatalf("hold shortened: %v < %v", c.backoffUntil, before)
	}
	held := c.backoffUntil
	c.backoffN = 0
	c.backoff(t0.Add(2 * time.Second)) // 1 s hold must not pull the hold in
	if c.backoffUntil.Before(held) {
		t.Fatalf("hold shortened by smaller back-off: %v < %v", c.backoffUntil, held)
	}
	c.success(held)
	if c.backoffN != 0 {
		t.Fatal("success after the hold should reset the exponent")
	}
}
