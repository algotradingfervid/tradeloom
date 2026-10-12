package clock

import (
	"sort"
	"sync"
	"time"
)

// Fake is a manually driven Clock for tests. Its system time only moves on Advance/Set; timers
// fire synchronously inside Advance/Set/SetOffset when the app time reaches their target.
type Fake struct {
	mu     sync.Mutex
	sys    time.Time
	off    Offset
	timers []*fakeTimer
}

// NewFake returns a Fake whose system time is now and whose offset is zero.
func NewFake(now time.Time) *Fake { return &Fake{sys: now, off: Offset{Source: SourceNone}} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sys.Add(f.off.Value).In(IST)
}

func (f *Fake) System() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sys.In(IST)
}

func (f *Fake) Offset() Offset {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.off
}

// SetOffset replaces the offset (Millis is derived) and fires due timers.
func (f *Fake) SetOffset(o Offset) {
	o.Millis = o.Value.Milliseconds()
	f.mu.Lock()
	f.off = o
	f.mu.Unlock()
	f.fire()
}

// Advance moves system time forward by d and fires due timers in target order.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.sys = f.sys.Add(d)
	f.mu.Unlock()
	f.fire()
}

// Set moves system time to t and fires due timers.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	f.sys = t
	f.mu.Unlock()
	f.fire()
}

func (f *Fake) At(target time.Time) Timer {
	t := &fakeTimer{f: f, target: target, ch: make(chan time.Time, 1)}
	f.mu.Lock()
	f.timers = append(f.timers, t)
	f.mu.Unlock()
	f.fire()
	return t
}

func (f *Fake) After(d time.Duration) Timer { return f.At(f.Now().Add(d)) }

// PendingTimers reports how many timers are armed and not yet fired or stopped.
func (f *Fake) PendingTimers() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.timers)
}

// WaitTimers blocks (real time, at most within) until at least n timers are pending.
// Tests use it to wait until a goroutine under test has armed its timer before Advance.
func (f *Fake) WaitTimers(n int, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if f.PendingTimers() >= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return f.PendingTimers() >= n
}

func (f *Fake) fire() {
	f.mu.Lock()
	now := f.sys.Add(f.off.Value).In(IST)
	var due, keep []*fakeTimer
	for _, t := range f.timers {
		if !now.Before(t.target) {
			due = append(due, t)
		} else {
			keep = append(keep, t)
		}
	}
	f.timers = keep
	f.mu.Unlock()
	sort.SliceStable(due, func(i, j int) bool { return due[i].target.Before(due[j].target) })
	for _, t := range due {
		select {
		case t.ch <- now:
		default:
		}
	}
}

type fakeTimer struct {
	f      *Fake
	target time.Time
	ch     chan time.Time
}

func (t *fakeTimer) C() <-chan time.Time { return t.ch }

func (t *fakeTimer) Stop() bool {
	t.f.mu.Lock()
	defer t.f.mu.Unlock()
	for i, x := range t.f.timers {
		if x == t {
			t.f.timers = append(t.f.timers[:i], t.f.timers[i+1:]...)
			return true
		}
	}
	return false
}
