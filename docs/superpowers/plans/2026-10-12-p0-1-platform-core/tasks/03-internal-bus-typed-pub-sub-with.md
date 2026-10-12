> **P0-1 Platform Core Implementation Plan — Task 3.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 3: `internal/bus` — typed pub/sub with bounded queues

**Files:**
- Create: `internal/bus/bus.go`, `internal/bus/bus_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces (`package bus`):
  - `const DefaultBuffer = 256`, `const CriticalBuffer = 4096`, `const CriticalWait = 50 * time.Millisecond`
  - `type Topic[T any] struct{…}`; `func NewTopic[T any](name string) Topic[T]`; `func (t Topic[T]) Name() string`
  - `type SubOpts[T any] struct { Name string; Buffer int; Critical bool; Coalesce func(old, new T) T }`
  - `type Stats struct { Topic string; Subscriber string; Queued int; Drops uint64 }`
  - `type Bus struct{…}`; `func New(log *slog.Logger) *Bus`; `func (b *Bus) Stats() []Stats`; `func (b *Bus) Close()`
  - `func Publish[T any](b *Bus, t Topic[T], v T)`; `func Subscribe[T any](b *Bus, t Topic[T], o SubOpts[T]) *Sub[T]`; `func Unsubscribe[T any](b *Bus, s *Sub[T])`
  - `type Sub[T any] struct{…}`; `func (s *Sub[T]) C() <-chan T`; `func (s *Sub[T]) Drops() uint64`
  - Semantics: Publish never blocks on a non-critical subscriber (full queue → drop oldest, or merge into the newest queued value with `Coalesce`); a full Critical queue makes Publish wait ≤ `CriticalWait`, log ERROR `bus.critical_queue_full`, then drop oldest; using one topic name with two payload types panics.

- [ ] **Step 1: Write the failing test**

`internal/bus/bus_test.go`:

```go
package bus

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type tick struct {
	Token uint32
	LTP   float64
}

var tickTopic = NewTopic[tick]("tick.test")

func recv[T any](t *testing.T, c <-chan T) T {
	t.Helper()
	select {
	case v, ok := <-c:
		if !ok {
			t.Fatal("channel closed")
		}
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
	var zero T
	return zero
}

func TestPublishSubscribe_FanOutInOrder(t *testing.T) {
	b := New(nil)
	defer b.Close()
	a := Subscribe(b, tickTopic, SubOpts[tick]{Name: "a"})
	c := Subscribe(b, tickTopic, SubOpts[tick]{Name: "c"})
	for i := 1; i <= 3; i++ {
		Publish(b, tickTopic, tick{Token: uint32(i)})
	}
	for _, s := range []*Sub[tick]{a, c} {
		for i := 1; i <= 3; i++ {
			if got := recv(t, s.C()); got.Token != uint32(i) {
				t.Fatalf("%s: want token %d got %d", s.opts.Name, i, got.Token)
			}
		}
	}
}

func TestSlowSubscriber_DropsOldestAndNeverBlocksPublisher(t *testing.T) {
	b := New(nil)
	defer b.Close()
	slow := Subscribe(b, tickTopic, SubOpts[tick]{Name: "slow", Buffer: 4})
	fast := Subscribe(b, tickTopic, SubOpts[tick]{Name: "fast", Buffer: 1024})
	var fastGot []uint32
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for v := range fast.C() {
			fastGot = append(fastGot, v.Token)
			if v.Token == 100 {
				return
			}
		}
	}()
	start := time.Now()
	for i := 1; i <= 100; i++ {
		Publish(b, tickTopic, tick{Token: uint32(i)})
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("publisher blocked for %v", d)
	}
	wg.Wait()
	if len(fastGot) != 100 {
		t.Fatalf("fast subscriber lost messages: %d", len(fastGot))
	}
	if slow.Drops() == 0 {
		t.Fatal("slow subscriber should have dropped")
	}
	// The newest message always survives.
	var last uint32
	for len(slow.C()) > 0 || last != 100 {
		last = recv(t, slow.C()).Token
	}
	st := b.Stats()
	if len(st) != 2 || st[1].Subscriber != "slow" || st[1].Drops != slow.Drops() {
		t.Fatalf("stats %+v", st)
	}
}

func TestCoalesce_MergesInsteadOfDropping(t *testing.T) {
	b := New(nil)
	defer b.Close()
	s := Subscribe(b, tickTopic, SubOpts[tick]{Name: "bridge", Buffer: 1,
		Coalesce: func(old, new tick) tick { return new }})
	for i := 1; i <= 50; i++ {
		Publish(b, tickTopic, tick{Token: 7, LTP: float64(i)})
	}
	var last tick
	deadline := time.After(2 * time.Second)
	for last.LTP != 50 {
		select {
		case last = <-s.C():
		case <-deadline:
			t.Fatalf("latest value never delivered, last=%v", last)
		}
	}
	if s.Drops() != 0 {
		t.Fatalf("coalescing subscriber counted %d drops", s.Drops())
	}
}

func TestCritical_WaitsThenLogsAndDrops(t *testing.T) {
	var logBuf bytes.Buffer
	b := New(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	defer b.Close()
	s := Subscribe(b, tickTopic, SubOpts[tick]{Name: "inbox", Critical: true, Buffer: 1})
	Publish(b, tickTopic, tick{Token: 1}) // taken by pump, blocked on unbuffered out
	Publish(b, tickTopic, tick{Token: 2}) // fills queue
	start := time.Now()
	Publish(b, tickTopic, tick{Token: 3}) // full: waits ≈ CriticalWait then drops oldest
	if d := time.Since(start); d < CriticalWait-5*time.Millisecond || d > 10*CriticalWait {
		t.Fatalf("critical publish waited %v, want ≈ %v", d, CriticalWait)
	}
	if !strings.Contains(logBuf.String(), "bus.critical_queue_full") {
		t.Fatalf("no ERROR logged: %s", logBuf.String())
	}
	if s.Drops() != 1 {
		t.Fatalf("drops=%d", s.Drops())
	}
}

func TestTopicTypeMismatchPanics(t *testing.T) {
	b := New(nil)
	defer b.Close()
	Subscribe(b, NewTopic[int]("x"), SubOpts[int]{Name: "a"})
	defer func() {
		if recover() == nil {
			t.Fatal("want panic for topic reused with another type")
		}
	}()
	Publish(b, NewTopic[string]("x"), "boom")
}

func TestCloseAndUnsubscribe(t *testing.T) {
	b := New(nil)
	s1 := Subscribe(b, tickTopic, SubOpts[tick]{Name: "s1"})
	s2 := Subscribe(b, tickTopic, SubOpts[tick]{Name: "s2"})
	Unsubscribe(b, s1)
	if _, ok := <-s1.C(); ok {
		t.Fatal("s1 channel should be closed")
	}
	b.Close()
	if _, ok := <-s2.C(); ok {
		t.Fatal("s2 channel should be closed after bus.Close")
	}
	Publish(b, tickTopic, tick{Token: 1}) // no panic
	s3 := Subscribe(b, tickTopic, SubOpts[tick]{Name: "late"})
	if _, ok := <-s3.C(); ok {
		t.Fatal("subscription on closed bus must be closed")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags noorders ./internal/bus/`
Expected: FAIL — build error `internal/bus/bus_test.go:17:17: undefined: NewTopic`.

- [ ] **Step 3: Write the implementation**

`internal/bus/bus.go`:

```go
// Package bus is Tradeloom's in-process typed pub/sub (ARCHITECTURE §3.4).
// Publish never blocks on a non-critical subscriber: each subscriber owns a
// bounded queue; when it is full the oldest message is dropped (or merged by
// Coalesce) and the subscriber's drop counter increments.
package bus

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultBuffer is the queue length of a non-critical subscriber.
	DefaultBuffer = 256
	// CriticalBuffer is the queue length of a Critical subscriber.
	CriticalBuffer = 4096
	// CriticalWait is how long Publish waits for space in a full Critical queue before dropping.
	CriticalWait = 50 * time.Millisecond
)

// Topic is a typed event name. Declare topics once as package-level vars: var T = bus.NewTopic[MyEvent]("my.event").
type Topic[T any] struct{ name string }

// NewTopic declares a topic. Names are mirrored 1:1 to Wails event names by internal/app.
func NewTopic[T any](name string) Topic[T] { return Topic[T]{name: name} }

// Name returns the topic name.
func (t Topic[T]) Name() string { return t.name }

// SubOpts configures a subscription.
type SubOpts[T any] struct {
	Name     string             // subscriber name used in Stats (required, e.g. "app.bridge")
	Buffer   int                // queue length; 0 means DefaultBuffer (CriticalBuffer when Critical)
	Critical bool               // publisher waits up to CriticalWait for space and logs ERROR on overflow
	Coalesce func(old, new T) T // when the queue is full, merge new into the newest queued value instead of dropping
}

// Stats is one subscriber's counters.
type Stats struct {
	Topic      string
	Subscriber string
	Queued     int
	Drops      uint64
}

type subscriber interface {
	deliver(v any)
	stats() Stats
	close()
}

// Bus is safe for concurrent use. The zero value is not usable; call New.
type Bus struct {
	log    *slog.Logger
	mu     sync.RWMutex
	subs   map[string][]subscriber
	types  map[string]reflect.Type
	closed bool
}

// New returns an empty bus. log receives ERROR records for critical overflows; nil uses slog.Default().
func New(log *slog.Logger) *Bus {
	if log == nil {
		log = slog.Default()
	}
	return &Bus{log: log, subs: map[string][]subscriber{}, types: map[string]reflect.Type{}}
}

func (b *Bus) checkType(name string, t reflect.Type) {
	if prev, ok := b.types[name]; ok && prev != t {
		panic(fmt.Sprintf("bus: topic %q used with %v and %v", name, prev, t))
	}
	b.types[name] = t
}

// Publish delivers v to every current subscriber of t. It never blocks longer than CriticalWait.
// Publishing on a closed bus is a no-op.
func Publish[T any](b *Bus, t Topic[T], v T) {
	subs, ok := b.snapshot(t.name, reflect.TypeFor[T]())
	if !ok {
		return
	}
	for _, s := range subs {
		s.deliver(v)
	}
}

func (b *Bus) snapshot(name string, typ reflect.Type) ([]subscriber, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, false
	}
	b.checkType(name, typ)
	return append([]subscriber(nil), b.subs[name]...), true
}

// Subscribe registers a subscriber and starts its delivery goroutine. Read events from Sub.C().
func Subscribe[T any](b *Bus, t Topic[T], o SubOpts[T]) *Sub[T] {
	if o.Buffer <= 0 {
		o.Buffer = DefaultBuffer
		if o.Critical {
			o.Buffer = CriticalBuffer
		}
	}
	s := &Sub[T]{
		topic: t.name, opts: o, log: b.log,
		notify: make(chan struct{}, 1), space: make(chan struct{}, 1),
		out: make(chan T), done: make(chan struct{}),
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.checkType(t.name, reflect.TypeFor[T]())
	if b.closed {
		close(s.out)
		return s
	}
	b.subs[t.name] = append(b.subs[t.name], s)
	go s.pump()
	return s
}

// Unsubscribe removes s from b and closes its channel.
func Unsubscribe[T any](b *Bus, s *Sub[T]) {
	b.mu.Lock()
	list := b.subs[s.topic]
	for i, x := range list {
		if x == subscriber(s) {
			b.subs[s.topic] = append(list[:i:i], list[i+1:]...)
			break
		}
	}
	b.mu.Unlock()
	s.close()
}

// Stats returns every subscriber's counters sorted by topic then subscriber.
func (b *Bus) Stats() []Stats {
	b.mu.RLock()
	var out []Stats
	for _, list := range b.subs {
		for _, s := range list {
			out = append(out, s.stats())
		}
	}
	b.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Topic != out[j].Topic {
			return out[i].Topic < out[j].Topic
		}
		return out[i].Subscriber < out[j].Subscriber
	})
	return out
}

// Close closes every subscription; later Publish calls are no-ops.
func (b *Bus) Close() {
	b.mu.Lock()
	b.closed = true
	all := b.subs
	b.subs = map[string][]subscriber{}
	b.mu.Unlock()
	for _, list := range all {
		for _, s := range list {
			s.close()
		}
	}
}

// Sub is one subscription. C() is closed when the subscription or the bus closes.
type Sub[T any] struct {
	topic  string
	opts   SubOpts[T]
	log    *slog.Logger
	mu     sync.Mutex
	q      []T
	notify chan struct{} // queue became non-empty
	space  chan struct{} // an item was taken from the queue
	out    chan T
	done   chan struct{}
	once   sync.Once
	drops  atomic.Uint64
}

// C returns the receive channel.
func (s *Sub[T]) C() <-chan T { return s.out }

// Drops returns how many messages this subscriber lost to back-pressure.
func (s *Sub[T]) Drops() uint64 { return s.drops.Load() }

func (s *Sub[T]) stats() Stats {
	s.mu.Lock()
	n := len(s.q)
	s.mu.Unlock()
	return Stats{Topic: s.topic, Subscriber: s.opts.Name, Queued: n, Drops: s.drops.Load()}
}

func (s *Sub[T]) signal(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}

func (s *Sub[T]) deliver(v any) {
	val := v.(T)
	s.mu.Lock()
	if len(s.q) < s.opts.Buffer {
		s.q = append(s.q, val)
		s.mu.Unlock()
		s.signal(s.notify)
		return
	}
	if s.opts.Coalesce != nil {
		last := len(s.q) - 1
		s.q[last] = s.opts.Coalesce(s.q[last], val)
		s.mu.Unlock()
		return
	}
	if s.opts.Critical {
		s.mu.Unlock()
		s.log.Error("bus.critical_queue_full", "component", "bus", "topic", s.topic, "subscriber", s.opts.Name)
		ctx, cancel := context.WithTimeout(context.Background(), CriticalWait)
		defer cancel()
		for {
			s.mu.Lock()
			if len(s.q) < s.opts.Buffer {
				s.q = append(s.q, val)
				s.mu.Unlock()
				s.signal(s.notify)
				return
			}
			s.mu.Unlock()
			select {
			case <-s.space:
				continue
			case <-s.done:
				return
			case <-ctx.Done():
			}
			break
		}
		s.mu.Lock()
	}
	// Drop the oldest queued message to make room for the newest.
	copy(s.q, s.q[1:])
	s.q[len(s.q)-1] = val
	s.mu.Unlock()
	s.drops.Add(1)
}

func (s *Sub[T]) pump() {
	defer close(s.out)
	for {
		s.mu.Lock()
		if len(s.q) == 0 {
			s.mu.Unlock()
			select {
			case <-s.notify:
				continue
			case <-s.done:
				return
			}
		}
		v := s.q[0]
		var zero T
		s.q[0] = zero
		s.q = s.q[1:]
		s.mu.Unlock()
		s.signal(s.space)
		select {
		case s.out <- v:
		case <-s.done:
			return
		}
	}
}

func (s *Sub[T]) close() { s.once.Do(func() { close(s.done) }) }
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race -count=5 -tags noorders ./internal/bus/ && go test -count=1 -tags noorders ./internal/archtest/`
Expected: `ok  tradeloom/internal/bus` and `ok  tradeloom/internal/archtest`.

- [ ] **Step 5: Commit**

```bash
git add internal/bus
git commit -m "feat(bus): typed pub/sub with bounded queues, coalescing, critical subscribers and drop counters" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
