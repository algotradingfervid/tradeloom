package ratelimit

import (
	"container/heap"
	"time"
)

// waiter is one queued request.
type waiter struct {
	prio  Priority
	seq   uint64
	ready chan struct{}
	index int // heap index; -1 once popped or removed
}

type waitHeap []*waiter

func (h waitHeap) Len() int { return len(h) }
func (h waitHeap) Less(i, j int) bool {
	if h[i].prio != h[j].prio {
		return h[i].prio < h[j].prio
	}
	return h[i].seq < h[j].seq
}
func (h waitHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *waitHeap) Push(x any)   { w := x.(*waiter); w.index = len(*h); *h = append(*h, w) }
func (h *waitHeap) Pop() any {
	old := *h
	n := len(old)
	w := old[n-1]
	old[n-1] = nil
	w.index = -1
	*h = old[:n-1]
	return w
}

// classState is the admission state of one class. It holds no lock and no goroutine; the
// Limiter guards it and drives it, and tests drive it directly with explicit times.
type classState struct {
	rate         int
	span         time.Duration
	grants       []time.Time // ring of the last `rate` grant times
	next         int
	count        int
	q            waitHeap
	backoffUntil time.Time
	backoffN     int
	http429      int64
	wake         chan struct{}
}

func newClassState(rate int, span time.Duration) *classState {
	return &classState{rate: rate, span: span, grants: make([]time.Time, rate), wake: make(chan struct{}, 1)}
}

// nextAllowed is the earliest instant a new grant keeps every span-long window at ≤ rate grants
// and respects the 429 back-off.
func (c *classState) nextAllowed() time.Time {
	t := c.backoffUntil
	if c.count == c.rate {
		if a := c.grants[c.next].Add(c.span); a.After(t) {
			t = a
		}
	}
	return t
}

// step grants the head waiter if allowed at now. It returns the granted waiter (nil if none) and,
// when the head must wait, how long.
func (c *classState) step(now time.Time, paused bool) (*waiter, time.Duration) {
	if paused || c.q.Len() == 0 {
		return nil, -1
	}
	if na := c.nextAllowed(); na.After(now) {
		return nil, na.Sub(now)
	}
	w := heap.Pop(&c.q).(*waiter)
	c.grants[c.next] = now
	c.next = (c.next + 1) % c.rate
	if c.count < c.rate {
		c.count++
	}
	return w, 0
}

// backoff applies the FR-DATA-003 429 policy: 1 s × 2ⁿ, capped at 30 s.
func (c *classState) backoff(now time.Time) time.Duration {
	d := time.Second << c.backoffN
	if d > 30*time.Second || d <= 0 {
		d = 30 * time.Second
	} else {
		c.backoffN++
	}
	c.backoffUntil = now.Add(d)
	c.http429++
	return d
}

func (c *classState) poke() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}
