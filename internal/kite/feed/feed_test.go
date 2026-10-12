// internal/kite/feed/feed_test.go
package feed

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/zerodha/gokiteconnect/v4/models"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ticker"
)

// fakeConn is a ticker.Conn that "connects" at once and records what the Manager asks of it.
type fakeConn struct {
	mu     sync.Mutex
	ev     ticker.Events
	tokens map[uint32]bool
	stops  int
}

func (c *fakeConn) Serve(ctx context.Context) {
	if c.ev.OnConnect != nil {
		c.ev.OnConnect()
	}
	<-ctx.Done()
}
func (c *fakeConn) Subscribe(tokens []uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range tokens {
		c.tokens[t] = true
	}
	return nil
}
func (c *fakeConn) Unsubscribe(tokens []uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range tokens {
		delete(c.tokens, t)
	}
	return nil
}
func (c *fakeConn) SetMode(ticker.Mode, []uint32) error { return nil }
func (c *fakeConn) Stop()                               { c.mu.Lock(); c.stops++; c.mu.Unlock() }
func (c *fakeConn) sorted() []uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]uint32, 0, len(c.tokens))
	for t := range c.tokens {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

type fakeDials struct {
	mu    sync.Mutex
	conns []*fakeConn
	creds []string
}

func (d *fakeDials) dialFor(apiKey, accessToken string) ticker.Dialer {
	d.mu.Lock()
	d.creds = append(d.creds, apiKey+":"+accessToken)
	d.mu.Unlock()
	return func(ev ticker.Events) ticker.Conn {
		c := &fakeConn{ev: ev, tokens: map[uint32]bool{}}
		d.mu.Lock()
		d.conns = append(d.conns, c)
		d.mu.Unlock()
		return c
	}
}

func (d *fakeDials) last() *fakeConn {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.conns) == 0 {
		return nil
	}
	return d.conns[len(d.conns)-1]
}

func (d *fakeDials) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.conns)
}

func (d *fakeDials) first() *fakeConn {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conns[0]
}

func (c *fakeConn) stopCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stops
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func newFeed(t *testing.T) (*Feed, *fakeDials) {
	t.Helper()
	d := &fakeDials{}
	f := New(Options{Ticker: ticker.Options{Clock: clock.NewFake(time.Date(2026, 10, 12, 9, 15, 0, 0, clock.IST)), PerConn: 4, MaxConns: 1}})
	f.dialFor = d.dialFor
	return f, d
}

func equal(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Subscriptions made before login are remembered and applied when the feed starts; Stop makes the feed
// refuse again without forgetting them (the next login replays them).
func TestFeed_RemembersAndReplaysAcrossLogins(t *testing.T) {
	f, d := newFeed(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := f.Subscribe(ticker.ConsumerWatchlist, "wl-1", []uint32{1, 2}, ticker.ModeQuote); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("before start: %v", err)
	}
	if u := f.Usage(); u.Used != 0 || u.Limit != 4 {
		t.Fatalf("usage before start %+v", u)
	}
	if err := f.Start(ctx, "key", "tok-1"); err != nil {
		t.Fatal(err)
	}
	if d.creds[0] != "key:tok-1" {
		t.Fatalf("dialer credentials %v", d.creds)
	}
	eventually(t, "replayed subscriptions", func() bool { c := d.last(); return c != nil && equal(c.sorted(), []uint32{1, 2}) })
	if err := f.Subscribe(ticker.ConsumerChart, "chart-1", []uint32{3}, ticker.ModeQuote); err != nil {
		t.Fatal(err)
	}
	eventually(t, "live subscribe", func() bool { return equal(d.last().sorted(), []uint32{1, 2, 3}) })
	if u := f.Usage(); u.Used != 3 {
		t.Fatalf("usage %+v", u)
	}
	var budget *ticker.ErrBudget
	if err := f.Subscribe(ticker.ConsumerChart, "chart-2", []uint32{4, 5}, ticker.ModeQuote); !errors.As(err, &budget) {
		t.Fatalf("over budget: %v", err)
	}
	f.Unsubscribe(ticker.ConsumerWatchlist, "wl-1", []uint32{2})
	f.Release(ticker.ConsumerChart, "chart-1")
	eventually(t, "unsubscribed", func() bool { return equal(d.last().sorted(), []uint32{1}) })

	f.Stop()
	if err := f.Subscribe(ticker.ConsumerWidget, "portfolio", []uint32{7}, ticker.ModeQuote); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("after stop: %v", err)
	}
	if u := f.Usage(); u.Used != 0 {
		t.Fatalf("usage after stop %+v", u)
	}
	if err := f.Start(ctx, "key", "tok-2"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "second login replay", func() bool { return d.count() == 2 && equal(d.last().sorted(), []uint32{1, 7}) })
	if d.first().stopCount() == 0 {
		t.Fatal("the first login's connection was not stopped")
	}
}

func TestFeed_TicksStreamSurvivesRestart(t *testing.T) {
	f, d := newFeed(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := f.Start(ctx, "key", "tok-1"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "connected", func() bool { return d.last() != nil })
	f.Stop()
	if err := f.Start(ctx, "key", "tok-2"); err != nil {
		t.Fatal(err)
	}
	if f.Ticks() == nil {
		t.Fatal("Ticks must be one long-lived channel")
	}
	select {
	case tk := <-f.Ticks():
		t.Fatalf("unexpected tick %+v", tk)
	case <-time.After(20 * time.Millisecond):
	}
}

// Ticks of each login's Manager reach Ticks(); after Stop closes the raw stream the pump exits and the
// next login's ticks still arrive on the same channel.
func TestFeed_PumpsTicksAcrossLogins(t *testing.T) {
	f, d := newFeed(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i, tok := range []string{"tok-1", "tok-2"} {
		if err := f.Start(ctx, "key", tok); err != nil {
			t.Fatal(err)
		}
		eventually(t, "connected", func() bool { c := d.last(); return c != nil && d.count() == i+1 })
		d.last().ev.OnTick(models.Tick{InstrumentToken: uint32(100 + i), LastPrice: 1})
		select {
		case tk := <-f.Ticks():
			if tk.Token != uint32(100+i) {
				t.Fatalf("login %d: tick %+v", i, tk)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("login %d: no tick pumped", i)
		}
		f.Stop()
	}
}
