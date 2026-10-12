package clock

import (
	"log/slog"
	"sync"
	"time"

	"tradeloom/internal/bus"
)

// TopicOffset carries every significant offset change (ARCHITECTURE §3.4 "clock.offset").
var TopicOffset = bus.NewTopic[Offset]("clock.offset")

// Options configures Real. Zero values take the FR-DATA-004 defaults.
type Options struct {
	Bus         *bus.Bus     // nil: no events
	Log         *slog.Logger // nil: slog.Default()
	System      func() time.Time
	TickWindow  time.Duration // default 60 s
	HTTPSamples int           // default 5
	WarnAbove   time.Duration // default 1 s
	SkewAbove   time.Duration // default 5 s
	ClearBelow  time.Duration // default 2 s
	Rearm       time.Duration // default 100 ms: offset moves larger than this re-arm timers
}

// Real is the production Clock. It also receives offset samples from the ticker manager
// (ObserveTick) and from the Kite HTTP transport (ObserveHTTPDate).
type Real struct {
	o     Options
	mu    sync.Mutex
	est   estimator
	off   Offset
	acked bool
	// armedValue is the offset the timers were last armed against; drift is measured from it.
	armedValue time.Duration
	// pubMu serialises recompute so events, logs and UpdatedAt stay ordered.
	pubMu  sync.Mutex
	timers map[*realTimer]struct{}
}

// New returns a Real clock with zero offset (source none).
func New(o Options) *Real {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.System == nil {
		o.System = SystemNow
	}
	if o.TickWindow == 0 {
		o.TickWindow = 60 * time.Second
	}
	if o.HTTPSamples == 0 {
		o.HTTPSamples = 5
	}
	if o.WarnAbove == 0 {
		o.WarnAbove = time.Second
	}
	if o.SkewAbove == 0 {
		o.SkewAbove = 5 * time.Second
	}
	if o.ClearBelow == 0 {
		o.ClearBelow = 2 * time.Second
	}
	if o.Rearm == 0 {
		o.Rearm = 100 * time.Millisecond
	}
	return &Real{
		o:      o,
		est:    estimator{window: o.TickWindow, httpN: o.HTTPSamples},
		off:    Offset{Source: SourceNone},
		timers: map[*realTimer]struct{}{},
	}
}

func (r *Real) Now() time.Time {
	r.mu.Lock()
	v := r.off.Value
	r.mu.Unlock()
	return r.o.System().Add(v).In(IST)
}

func (r *Real) System() time.Time { return r.o.System().In(IST) }

func (r *Real) Offset() Offset {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.off
}

// ObserveTick records one exchange timestamp (models.Tick.Timestamp of the clock reference
// token) and its system arrival time.
func (r *Real) ObserveTick(exchangeTS, arrival time.Time) {
	if exchangeTS.IsZero() {
		return
	}
	r.mu.Lock()
	r.est.addTick(exchangeTS, arrival)
	r.mu.Unlock()
	r.recompute()
}

// ObserveHTTPDate records a Kite HTTP Date header with the system times the request was sent
// and the response received.
func (r *Real) ObserveHTTPDate(date, sent, received time.Time) {
	if date.IsZero() || received.Before(sent) {
		return
	}
	r.mu.Lock()
	r.est.addHTTP(date, sent, received)
	r.mu.Unlock()
	r.recompute()
}

// Acknowledge clears Skewed until the offset next returns below ClearBelow and rises again.
// It is a no-op unless the clock is currently skewed, so it cannot pre-empt a later skew.
func (r *Real) Acknowledge() {
	r.mu.Lock()
	if !r.off.Skewed {
		r.mu.Unlock()
		return
	}
	r.acked = true
	r.mu.Unlock()
	r.recompute()
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func (r *Real) recompute() {
	r.pubMu.Lock()
	defer r.pubMu.Unlock()
	sys := r.o.System()
	r.mu.Lock()
	v, src, n := r.est.estimate(sys)
	prev := r.off
	a := abs(v)
	if a < r.o.ClearBelow {
		r.acked = false
	}
	skewed := prev.Skewed
	if a > r.o.SkewAbove {
		skewed = true
	}
	if a < r.o.ClearBelow || r.acked {
		skewed = false
	}
	next := Offset{
		Value: v, Millis: v.Milliseconds(), Source: src, Samples: n,
		Warn: a > r.o.WarnAbove, Skewed: skewed, UpdatedAt: sys.Add(v).In(IST),
	}
	r.off = next
	significant := abs(v-r.armedValue) > r.o.Rearm || src != prev.Source ||
		next.Warn != prev.Warn || next.Skewed != prev.Skewed
	var ts []*realTimer
	if significant {
		r.armedValue = v
		for t := range r.timers {
			ts = append(ts, t)
		}
	}
	r.mu.Unlock()
	if !significant {
		return
	}
	for _, t := range ts {
		t.arm()
	}
	switch {
	case next.Skewed && !prev.Skewed:
		r.o.Log.Error("clock out of sync; live evaluation must suspend", "offset_ms", next.Millis, "source", string(src))
	case next.Warn && !prev.Warn:
		r.o.Log.Warn("clock offset above 1 s", "offset_ms", next.Millis, "source", string(src))
	default:
		r.o.Log.Info("clock offset", "offset_ms", next.Millis, "source", string(src), "samples", n)
	}
	publish(r.o.Bus, TopicOffset, next)
}

func (r *Real) At(target time.Time) Timer {
	t := &realTimer{r: r, target: target, ch: make(chan time.Time, 1)}
	r.mu.Lock()
	r.timers[t] = struct{}{}
	r.mu.Unlock()
	t.arm()
	return t
}

// After fires d later in exchange time (Now()-relative), not system time.
func (r *Real) After(d time.Duration) Timer { return r.At(r.Now().Add(d)) }

func (r *Real) forget(t *realTimer) {
	r.mu.Lock()
	delete(r.timers, t)
	r.mu.Unlock()
}

type realTimer struct {
	r      *Real
	target time.Time
	ch     chan time.Time
	mu     sync.Mutex
	af     *time.Timer
	done   bool
}

func (t *realTimer) C() <-chan time.Time { return t.ch }

// arm (re)schedules the timer for target − Now(); called on creation and on offset moves.
func (t *realTimer) arm() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return
	}
	d := t.target.Sub(t.r.Now())
	if d < 0 {
		d = 0
	}
	if t.af != nil {
		t.af.Stop()
	}
	t.af = time.AfterFunc(d, t.check)
}

func (t *realTimer) check() {
	now := t.r.Now()
	if now.Before(t.target) { // the offset moved backwards since arming
		t.arm()
		return
	}
	t.mu.Lock()
	if t.done {
		t.mu.Unlock()
		return
	}
	t.done = true
	t.mu.Unlock()
	t.r.forget(t)
	select {
	case t.ch <- now:
	default:
	}
}

func (t *realTimer) Stop() bool {
	t.mu.Lock()
	if t.done {
		t.mu.Unlock()
		return false
	}
	t.done = true
	if t.af != nil {
		t.af.Stop()
	}
	t.mu.Unlock()
	t.r.forget(t)
	return true
}

// publish sends v on t when a bus is configured (tests and tools may run without one).
func publish[T any](b *bus.Bus, t bus.Topic[T], v T) {
	if b != nil {
		bus.Publish(b, t, v)
	}
}
