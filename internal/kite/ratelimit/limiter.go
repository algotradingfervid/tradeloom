package ratelimit

import (
	"container/heap"
	"context"
	"log/slog"
	"sync"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/logging"
)

// TopicUsage carries the 1 Hz usage snapshot (ARCHITECTURE §3.4 "limits.usage").
var TopicUsage = bus.NewTopic[Usage]("limits.usage")

// ClassUsage is one class's queue and back-off state.
type ClassUsage struct {
	Class        string             `json:"class"`
	RatePerSec   int                `json:"ratePerSec"`
	Queued       [NumPriorities]int `json:"queued"` // by Priority
	HTTP429      int64              `json:"http429"`
	BackoffUntil time.Time          `json:"backoffUntil"`
}

// Usage is the snapshot shown in Settings → Limits (FR-ACCT-009).
type Usage struct {
	At          time.Time    `json:"at"`
	Paused      bool         `json:"paused"`
	PauseReason string       `json:"pauseReason"`
	Classes     []ClassUsage `json:"classes"`
}

// Options configures a Limiter.
type Options struct {
	Rates  map[Class]int // nil: DefaultRates
	Margin time.Duration // added to the 1 s window; default 20 ms
	Bus    *bus.Bus
	Log    *slog.Logger
}

// Limiter is shared by every Kite HTTP call.
type Limiter struct {
	clk    clock.Clock
	o      Options
	mu     sync.Mutex
	cls    [numClasses]*classState
	seq    uint64
	paused bool
	reason string
}

// New builds a Limiter; call Run to start its schedulers.
func New(clk clock.Clock, o Options) *Limiter {
	if o.Rates == nil {
		o.Rates = DefaultRates
	}
	if o.Margin == 0 {
		o.Margin = 20 * time.Millisecond
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	l := &Limiter{clk: clk, o: o}
	for c := Class(0); c < numClasses; c++ {
		r := o.Rates[c]
		if r <= 0 {
			r = DefaultRates[c]
		}
		l.cls[c] = newClassState(r, time.Second+o.Margin)
	}
	return l
}

// Run starts one scheduler goroutine per class and the 1 Hz usage publisher; they stop with ctx.
func (l *Limiter) Run(ctx context.Context) {
	for c := Class(0); c < numClasses; c++ {
		c := c
		logging.Go("ratelimit."+c.String(), func() { l.loop(ctx, c) })
	}
	logging.Go("ratelimit.usage", func() {
		for {
			if err := clock.Sleep(ctx, l.clk, time.Second); err != nil {
				return
			}
			publish(l.o.Bus, TopicUsage, l.Snapshot())
		}
	})
}

// Acquire blocks until class c grants a slot to a request of priority p, or ctx ends.
func (l *Limiter) Acquire(ctx context.Context, c Class, p Priority) error {
	if p >= NumPriorities {
		p = NumPriorities - 1
	}
	w := &waiter{prio: p, ready: make(chan struct{})}
	cs := l.cls[c]
	l.mu.Lock()
	l.seq++
	w.seq = l.seq
	heap.Push(&cs.q, w)
	l.mu.Unlock()
	cs.poke()
	select {
	case <-w.ready:
		return nil
	case <-ctx.Done():
		l.mu.Lock()
		if w.index >= 0 {
			heap.Remove(&cs.q, w.index)
		}
		l.mu.Unlock()
		return ctx.Err()
	}
}

// Do acquires a slot and runs fn.
func (l *Limiter) Do(ctx context.Context, c Class, p Priority, fn func() error) error {
	if err := l.Acquire(ctx, c, p); err != nil {
		return err
	}
	return fn()
}

// Backoff records an HTTP 429 on class c and delays only that class (FR-DATA-003 AC2).
func (l *Limiter) Backoff(c Class) {
	l.mu.Lock()
	d := l.cls[c].backoff(l.clk.Now())
	n := l.cls[c].http429
	l.mu.Unlock()
	l.o.Log.Info("kite rate limited", "class", c.String(), "backoff", d.String(), "http429_total", n)
}

// Success resets class c's back-off exponent after a successful response.
func (l *Limiter) Success(c Class) {
	l.mu.Lock()
	l.cls[c].backoffN = 0
	l.mu.Unlock()
}

// PauseAll stops every class from granting (TokenException, FR-DATA-003 AC4).
func (l *Limiter) PauseAll(reason string) {
	l.mu.Lock()
	already := l.paused
	l.paused, l.reason = true, reason
	l.mu.Unlock()
	if !already {
		l.o.Log.Info("kite queues paused", "reason", reason)
	}
}

// Resume lets every class grant again (after login).
func (l *Limiter) Resume() {
	l.mu.Lock()
	l.paused, l.reason = false, ""
	l.mu.Unlock()
	for _, cs := range l.cls {
		cs.poke()
	}
	l.o.Log.Info("kite queues resumed")
}

// Snapshot returns current usage.
func (l *Limiter) Snapshot() Usage {
	l.mu.Lock()
	defer l.mu.Unlock()
	u := Usage{At: l.clk.Now(), Paused: l.paused, PauseReason: l.reason}
	for c := Class(0); c < numClasses; c++ {
		cs := l.cls[c]
		cu := ClassUsage{Class: c.String(), RatePerSec: cs.rate, HTTP429: cs.http429, BackoffUntil: cs.backoffUntil}
		for _, w := range cs.q {
			cu.Queued[w.prio]++
		}
		u.Classes = append(u.Classes, cu)
	}
	return u
}

func (l *Limiter) loop(ctx context.Context, c Class) {
	cs := l.cls[c]
	for {
		l.mu.Lock()
		w, wait := cs.step(l.clk.Now(), l.paused)
		l.mu.Unlock()
		if w != nil {
			close(w.ready)
			continue
		}
		var tc <-chan time.Time
		var t clock.Timer
		if wait > 0 {
			t = l.clk.After(wait)
			tc = t.C()
		}
		select {
		case <-ctx.Done():
			if t != nil {
				t.Stop()
			}
			return
		case <-cs.wake:
		case <-tc:
		}
		if t != nil {
			t.Stop()
		}
	}
}

// publish sends v on t when a bus is configured (tests and tools may run without one).
func publish[T any](b *bus.Bus, t bus.Topic[T], v T) {
	if b != nil {
		bus.Publish(b, t, v)
	}
}
