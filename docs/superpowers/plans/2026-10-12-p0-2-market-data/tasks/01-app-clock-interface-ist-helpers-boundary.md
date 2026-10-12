> **P0-2 Time & Market Data Implementation Plan — Task 1.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 1: App clock interface, IST helpers, boundary math, fake clock

**Files:**
- Create: `internal/clock/clock.go`
- Create: `internal/clock/ist.go`
- Create: `internal/clock/fake.go`
- Test: `internal/clock/clock_test.go`

**Interfaces:**
- Consumes: P0-1 Task 2: `internal/clock/system.go` with `func SystemNow() time.Time` already exists in the package and stays unchanged. Standard library only (`time/tzdata` embedded).
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
var IST = mustLoadIST()
type Source string
const (
	SourceNone     Source = "none"
	SourceTicks    Source = "ticks"
	SourceHTTPDate Source = "http-date"
)
type Offset struct {
	Value     time.Duration `json:"-"`
	Millis    int64         `json:"millis"`
	Source    Source        `json:"source"`
	Samples   int           `json:"samples"`
	Warn      bool          `json:"warn"`   // |Value| > 1 s
	Skewed    bool          `json:"skewed"` // |Value| > 5 s and not yet back under 2 s or acknowledged
	UpdatedAt time.Time     `json:"updatedAt"`
}
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}
type Clock interface {
	Now() time.Time    // app clock (system + offset), in IST
	System() time.Time // raw system time in IST; only for stamping offset samples
	Offset() Offset
	At(target time.Time) Timer   // fires when Now() >= target; re-armed when the offset moves
	After(d time.Duration) Timer // At(Now()+d)
}
func Sleep(ctx context.Context, c Clock, d time.Duration) error
func MidnightIST(t time.Time) time.Time
func FormatIST(t time.Time) string
func FormatDateIST(t time.Time) string
func NextBoundary(now, anchor time.Time, step time.Duration) time.Time
func BucketStart(t, anchor time.Time, step time.Duration) time.Time
type Fake struct{ /* unexported fields */ }
func NewFake(now time.Time) *Fake
func (f *Fake) Now() time.Time
func (f *Fake) System() time.Time
func (f *Fake) Offset() Offset
func (f *Fake) SetOffset(o Offset)
func (f *Fake) Advance(d time.Duration)
func (f *Fake) Set(t time.Time)
func (f *Fake) At(target time.Time) Timer
func (f *Fake) After(d time.Duration) Timer
func (f *Fake) PendingTimers() int
func (f *Fake) WaitTimers(n int, within time.Duration) bool
```

Everything that schedules receives a `clock.Clock`; `Fake` makes every later test deterministic. AC2 of FR-DATA-004 (IST regardless of system zone; 06:00 IST expiry fires at 00:30 UTC) is proved here.

- [ ] **Step 1: Write the failing tests**

`internal/clock/clock_test.go`:

```go
package clock

import (
	"context"
	"testing"
	"time"
)

func TestISTIsKolkataEvenWhenLocalIsUTC(t *testing.T) {
	saved := time.Local
	time.Local = time.UTC
	defer func() { time.Local = saved }()

	f := NewFake(time.Date(2026, 10, 13, 0, 30, 0, 0, time.UTC))
	if got := f.Now().Location().String(); got != "Asia/Kolkata" {
		t.Fatalf("Now() zone = %q, want Asia/Kolkata", got)
	}
	if got := f.Now().Format("15:04"); got != "06:00" {
		t.Fatalf("00:30 UTC rendered as %s IST, want 06:00", got)
	}
}

func TestExpiryAt0600ISTFiresAt0030UTC(t *testing.T) { // FR-DATA-004 AC2
	f := NewFake(time.Date(2026, 10, 13, 0, 29, 59, 0, time.UTC))
	expiry := time.Date(2026, 10, 13, 6, 0, 0, 0, IST)
	tm := f.At(expiry)
	select {
	case <-tm.C():
		t.Fatal("fired one second early")
	default:
	}
	f.Advance(time.Second)
	select {
	case got := <-tm.C():
		if got.UTC().Format("15:04:05") != "00:30:00" {
			t.Fatalf("fired at %s UTC, want 00:30:00", got.UTC().Format("15:04:05"))
		}
	default:
		t.Fatal("did not fire at 00:30:00 UTC")
	}
}

func TestNextBoundaryAndBucketStart(t *testing.T) {
	anchor := time.Date(2026, 10, 12, 9, 15, 0, 0, IST)
	cases := []struct {
		now  time.Time
		step time.Duration
		next string
	}{
		{time.Date(2026, 10, 12, 9, 0, 0, 0, IST), 5 * time.Minute, "09:15:00"},
		{anchor, 5 * time.Minute, "09:20:00"},
		{time.Date(2026, 10, 12, 9, 19, 59, 0, IST), 5 * time.Minute, "09:20:00"},
		{time.Date(2026, 10, 12, 10, 14, 0, 0, IST), time.Hour, "10:15:00"},
		{time.Date(2026, 10, 12, 10, 15, 0, 0, IST), time.Hour, "11:15:00"},
	}
	for _, c := range cases {
		if got := NextBoundary(c.now, anchor, c.step).Format("15:04:05"); got != c.next {
			t.Errorf("NextBoundary(%s, %v) = %s, want %s", c.now.Format("15:04:05"), c.step, got, c.next)
		}
	}
	if got := BucketStart(time.Date(2026, 10, 12, 10, 2, 30, 0, IST), anchor, 5*time.Minute).Format("15:04"); got != "10:00" {
		t.Fatalf("BucketStart = %s, want 10:00", got)
	}
}

func TestFakeOffsetMovesNowNotSystem(t *testing.T) {
	sys := time.Date(2026, 10, 12, 10, 0, 3, 0, IST)
	f := NewFake(sys)
	f.SetOffset(Offset{Value: -3 * time.Second, Source: SourceTicks})
	if !f.System().Equal(sys) {
		t.Fatalf("System moved: %v", f.System())
	}
	if got := f.Now().Format("15:04:05"); got != "10:00:00" {
		t.Fatalf("Now = %s, want 10:00:00", got)
	}
	if f.Offset().Millis != -3000 {
		t.Fatalf("Millis = %d", f.Offset().Millis)
	}
}

func TestFakeStopAndSleep(t *testing.T) {
	f := NewFake(time.Date(2026, 10, 12, 10, 0, 0, 0, IST))
	tm := f.After(time.Minute)
	if !tm.Stop() {
		t.Fatal("Stop on pending timer returned false")
	}
	f.Advance(time.Hour)
	select {
	case <-tm.C():
		t.Fatal("stopped timer fired")
	default:
	}

	done := make(chan error, 1)
	go func() { done <- Sleep(context.Background(), f, 10*time.Second) }()
	if !f.WaitTimers(1, time.Second) {
		t.Fatal("Sleep never armed a timer")
	}
	f.Advance(10 * time.Second)
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Sleep(ctx, f, time.Hour); err != context.Canceled {
		t.Fatalf("Sleep on cancelled ctx = %v", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/clock/`
Expected: the build fails — output contains `undefined: NewFake` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/clock/clock.go`:

```go
// Package clock is Tradeloom's app clock: system time plus a measured offset to exchange
// time (FR-DATA-004). It is the only package that may call time.Now, time.Sleep, time.After,
// time.AfterFunc or time.NewTimer (ARCHITECTURE §2.4 rule 8); every other package receives a
// Clock and gets time from it.
package clock

import (
	"context"
	"time"
	_ "time/tzdata" // embed the zone database so IST resolves on minimal systems
)

// IST is Asia/Kolkata. Every time the app shows, logs or schedules is in this zone.
var IST = mustLoadIST()

func mustLoadIST() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		panic("clock: cannot load Asia/Kolkata: " + err.Error())
	}
	return loc
}

// Source says where the current offset estimate comes from.
type Source string

const (
	SourceNone     Source = "none"
	SourceTicks    Source = "ticks"
	SourceHTTPDate Source = "http-date"
)

// Offset is the measured difference exchange time − system time. App time = system + Value.
type Offset struct {
	Value     time.Duration `json:"-"`
	Millis    int64         `json:"millis"`
	Source    Source        `json:"source"`
	Samples   int           `json:"samples"`
	Warn      bool          `json:"warn"`   // |Value| > 1 s
	Skewed    bool          `json:"skewed"` // |Value| > 5 s and not yet back under 2 s or acknowledged
	UpdatedAt time.Time     `json:"updatedAt"`
}

// Timer fires once on C when the app clock reaches its target.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// Clock is the interface every package receives. Real is the production implementation,
// Fake the test implementation.
type Clock interface {
	Now() time.Time    // app clock (system + offset), in IST
	System() time.Time // raw system time in IST; only for stamping offset samples
	Offset() Offset
	At(target time.Time) Timer   // fires when Now() >= target; re-armed when the offset moves
	After(d time.Duration) Timer // At(Now()+d)
}

// Sleep waits d on c or until ctx ends.
func Sleep(ctx context.Context, c Clock, d time.Duration) error {
	t := c.After(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C():
		return nil
	}
}
```

`internal/clock/ist.go`:

```go
package clock

import "time"

// MidnightIST returns 00:00 IST of the IST calendar day containing t.
func MidnightIST(t time.Time) time.Time {
	y, m, d := t.In(IST).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, IST)
}

// FormatIST renders t as DD-MM-YYYY HH:MM:SS in IST (NFR-11 date format).
func FormatIST(t time.Time) string { return t.In(IST).Format("02-01-2006 15:04:05") }

// FormatDateIST renders t as DD-MM-YYYY in IST.
func FormatDateIST(t time.Time) string { return t.In(IST).Format("02-01-2006") }

// NextBoundary returns the first instant anchor + k·step that is strictly after now
// (anchor itself when now is before anchor). step must be > 0.
func NextBoundary(now, anchor time.Time, step time.Duration) time.Time {
	if step <= 0 {
		panic("clock: NextBoundary step must be > 0")
	}
	if now.Before(anchor) {
		return anchor
	}
	k := now.Sub(anchor)/step + 1
	return anchor.Add(k * step)
}

// BucketStart returns anchor + floor((t−anchor)/step)·step. t must not be before anchor.
func BucketStart(t, anchor time.Time, step time.Duration) time.Time {
	if step <= 0 {
		panic("clock: BucketStart step must be > 0")
	}
	k := t.Sub(anchor) / step
	return anchor.Add(k * step)
}
```

`internal/clock/fake.go`:

```go
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/clock/`
Expected: `ok  	tradeloom/internal/clock`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/clock/clock.go \
  internal/clock/ist.go \
  internal/clock/fake.go \
  internal/clock/clock_test.go
git commit -m "$(cat <<'EOF'
feat(clock): app clock interface, IST helpers, boundary math and fake clock

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
