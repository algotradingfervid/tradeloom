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
		if s.waitForSpace(val) {
			return
		}
	} else {
		s.mu.Unlock()
	}
	s.overflow(val)
}

// waitForSpace waits up to CriticalWait for room in a full Critical queue and
// enqueues val if room appears. It reports whether val was enqueued (or the
// subscription closed); false means the caller must drop.
func (s *Sub[T]) waitForSpace(val T) bool {
	ctx, cancel := context.WithTimeout(context.Background(), CriticalWait)
	defer cancel()
	for {
		s.mu.Lock()
		if len(s.q) < s.opts.Buffer {
			s.q = append(s.q, val)
			s.mu.Unlock()
			s.signal(s.notify)
			return true
		}
		s.mu.Unlock()
		select {
		case <-s.space:
		case <-s.done:
			return true
		case <-ctx.Done():
			return false
		}
	}
}

// overflow makes room for val by dropping the oldest queued message. If the
// queue drained in the meantime val is simply enqueued and nothing is dropped.
// A Critical subscriber logs ERROR only when a message is actually dropped.
func (s *Sub[T]) overflow(val T) {
	s.mu.Lock()
	if len(s.q) < s.opts.Buffer {
		s.q = append(s.q, val)
		s.mu.Unlock()
		s.signal(s.notify)
		return
	}
	copy(s.q, s.q[1:])
	s.q[len(s.q)-1] = val
	s.mu.Unlock()
	s.drops.Add(1)
	if s.opts.Critical {
		s.log.Error("bus.critical_queue_full", "component", "bus", "topic", s.topic, "subscriber", s.opts.Name)
	}
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
