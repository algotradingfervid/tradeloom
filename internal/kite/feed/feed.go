// internal/kite/feed/feed.go
// Package feed owns one ticker.Manager per Kite login. A Manager is single-use (P0-2: Stop is final and
// carries the access token in its dialer), so the Session starts a fresh one after every login while
// consumers keep subscribing to this long-lived Feed; their subscriptions are replayed on each Start.
package feed

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"sort"
	"sync"

	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/logging"
)

// ErrNotRunning is returned by Subscribe while no login is active. The subscription is still
// remembered and applied at the next Start; callers that need data now fall back to polling.
var ErrNotRunning = errors.New("feed: not running (no Kite session)")

const rawBuffer = 4096

// Options configures the Feed. Ticker.Dial is ignored: every Start builds ticker.KiteDialer with that login's credentials.
type Options struct {
	Ticker ticker.Options
	Root   *url.URL // nil = Kite's production WebSocket root; tests pass mock.Server.TickerURL()
	Log    *slog.Logger
}

type ownerKey struct {
	c     ticker.Consumer
	owner string
}

// Feed is safe for concurrent use.
type Feed struct {
	o       Options
	dialFor func(apiKey, accessToken string) ticker.Dialer
	ticks   chan ticker.Tick

	mu       sync.Mutex
	m        *ticker.Manager
	stopPump context.CancelFunc
	want     map[ownerKey]map[uint32]ticker.Mode
}

// New builds a stopped Feed.
func New(o Options) *Feed {
	if o.Log == nil {
		o.Log = o.Ticker.Log
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	f := &Feed{o: o, ticks: make(chan ticker.Tick, rawBuffer), want: map[ownerKey]map[uint32]ticker.Mode{}}
	f.dialFor = func(apiKey, accessToken string) ticker.Dialer { return ticker.KiteDialer(apiKey, accessToken, o.Root) }
	return f
}

// Start builds and starts a Manager for this login, pumps its raw ticks into Ticks() and replays every
// remembered subscription. A previous Manager is stopped first.
func (f *Feed) Start(ctx context.Context, apiKey, accessToken string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopLocked()
	opts := f.o.Ticker
	opts.Dial = f.dialFor(apiKey, accessToken)
	if opts.Log == nil {
		opts.Log = f.o.Log
	}
	m := ticker.New(opts)
	if err := m.Start(ctx); err != nil {
		m.Stop()
		return err
	}
	f.m = m
	raw, cancelRaw := m.Raw(rawBuffer)
	pumpCtx, cancelPump := context.WithCancel(ctx)
	f.stopPump = func() { cancelRaw(); cancelPump() }
	logging.Go("feed.pump", func() {
		for {
			select {
			case <-pumpCtx.Done():
				return
			case t, ok := <-raw:
				if !ok { // the Manager closed the raw stream (cancelRaw)
					return
				}
				select {
				case f.ticks <- t:
				default:
				}
			}
		}
	})
	for k, set := range f.want {
		for _, mode := range []ticker.Mode{ticker.ModeLTP, ticker.ModeQuote, ticker.ModeFull} {
			toks := tokensAt(set, mode)
			if len(toks) == 0 {
				continue
			}
			if err := m.Subscribe(k.c, k.owner, toks, mode); err != nil {
				f.o.Log.Warn("feed.replay_refused", "component", "feed", "consumer", k.c.String(), "owner", k.owner, "tokens", len(toks), "err", err)
			}
		}
	}
	return nil
}

func tokensAt(set map[uint32]ticker.Mode, mode ticker.Mode) []uint32 {
	var out []uint32
	for t, m := range set {
		if m == mode {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Stop ends the current login's Manager (logout, expiry, shutdown). Subscriptions stay remembered.
func (f *Feed) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopLocked()
}

func (f *Feed) stopLocked() {
	if f.stopPump != nil {
		f.stopPump()
		f.stopPump = nil
	}
	if f.m != nil {
		f.m.Stop()
		f.m = nil
	}
}

// Reconnect is the user's "Retry now" after the Manager went offline.
func (f *Feed) Reconnect() {
	f.mu.Lock()
	m := f.m
	f.mu.Unlock()
	if m != nil {
		m.Reconnect()
	}
}

// Subscribe remembers tokens for (c, owner) at mode and forwards them to the running Manager. A budget
// refusal (*ticker.ErrBudget) changes nothing; when stopped it returns ErrNotRunning after remembering.
func (f *Feed) Subscribe(c ticker.Consumer, owner string, tokens []uint32, mode ticker.Mode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m != nil {
		if err := f.m.Subscribe(c, owner, tokens, mode); err != nil {
			return err
		}
	}
	k := ownerKey{c, owner}
	set := f.want[k]
	if set == nil {
		set = map[uint32]ticker.Mode{}
		f.want[k] = set
	}
	for _, t := range tokens {
		if set[t] < mode {
			set[t] = mode
		}
	}
	if f.m == nil {
		return ErrNotRunning
	}
	return nil
}

// Unsubscribe forgets tokens of (c, owner) and forwards when running.
func (f *Feed) Unsubscribe(c ticker.Consumer, owner string, tokens []uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if set := f.want[ownerKey{c, owner}]; set != nil {
		for _, t := range tokens {
			delete(set, t)
		}
	}
	if f.m != nil {
		f.m.Unsubscribe(c, owner, tokens)
	}
}

// Release forgets every token of (c, owner) and forwards when running.
func (f *Feed) Release(c ticker.Consumer, owner string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.want, ownerKey{c, owner})
	if f.m != nil {
		f.m.Release(c, owner)
	}
}

// Usage is the running Manager's budget, or an empty budget of the configured size when stopped.
func (f *Feed) Usage() ticker.Usage {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m != nil {
		return f.m.Usage()
	}
	perConn, maxConns := f.o.Ticker.PerConn, f.o.Ticker.MaxConns
	if perConn == 0 {
		perConn = 3000
	}
	if maxConns == 0 {
		maxConns = 3
	}
	return ticker.Usage{Limit: perConn * maxConns, ByConsumer: map[string]int{}}
}

// Ticks is the raw tick stream across logins (the candle builder's Run input). A full buffer drops ticks.
func (f *Feed) Ticks() <-chan ticker.Tick { return f.ticks }
