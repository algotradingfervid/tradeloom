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

// Ruling 5: the overflow path must tolerate the queue emptying during the critical wait.
func TestCritical_QueueEmptiedDuringWaitDoesNotPanic(t *testing.T) {
	b := New(nil)
	defer b.Close()
	s := Subscribe(b, tickTopic, SubOpts[tick]{Name: "inbox", Critical: true, Buffer: 1})
	// Simulate the queue draining between the last capacity check and the overflow handling.
	s.mu.Lock()
	s.q = nil
	s.mu.Unlock()
	s.overflow(tick{Token: 9}) // must not panic
	if got := recv(t, s.C()); got.Token != 9 {
		t.Fatalf("want token 9 enqueued, got %d", got.Token)
	}
	if s.Drops() != 0 {
		t.Fatalf("nothing was dropped, drops=%d", s.Drops())
	}
}

func TestCritical_NoErrorLoggedWhenSpaceFreesInTime(t *testing.T) {
	var logBuf bytes.Buffer
	b := New(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	defer b.Close()
	s := Subscribe(b, tickTopic, SubOpts[tick]{Name: "inbox", Critical: true, Buffer: 1})
	Publish(b, tickTopic, tick{Token: 1}) // held by pump on unbuffered out
	Publish(b, tickTopic, tick{Token: 2}) // fills queue
	go func() {
		time.Sleep(10 * time.Millisecond)
		recv(t, s.C()) // frees the pump, which takes token 2 off the queue
	}()
	Publish(b, tickTopic, tick{Token: 3}) // waits, then succeeds without dropping
	if s.Drops() != 0 {
		t.Fatalf("drops=%d, want 0", s.Drops())
	}
	if strings.Contains(logBuf.String(), "bus.critical_queue_full") {
		t.Fatalf("ERROR logged although nothing was dropped: %s", logBuf.String())
	}
}
