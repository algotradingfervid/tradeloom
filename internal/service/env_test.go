// internal/service/env_test.go
package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/candles"
	"tradeloom/internal/candles/backfill"
	"tradeloom/internal/clock"
	"tradeloom/internal/instruments"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/orders"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

type fakeSession struct {
	mu        sync.Mutex
	st        kite.SessionState
	err       error
	completed []string
	forwarded []string
	profile   kite.Profile
}

func (f *fakeSession) State() kite.SessionState      { f.mu.Lock(); defer f.mu.Unlock(); return f.st }
func (f *fakeSession) setState(st kite.SessionState) { f.mu.Lock(); defer f.mu.Unlock(); f.st = st }
func (f *fakeSession) CredentialsInfo() kite.CredentialsInfo {
	return kite.CredentialsInfo{Configured: true, APIKeyMasked: "••••abcd", Port: 7420, RedirectURL: kite.RedirectURL(7420), SecretBackend: "memory"}
}
func (f *fakeSession) SaveCredentials(context.Context, string, string) (kite.SessionState, error) {
	return f.State(), f.err
}
func (f *fakeSession) RemoveCredentials(context.Context) (kite.SessionState, error) {
	return f.State(), f.err
}
func (f *fakeSession) SetRedirectPort(int) (kite.CredentialsInfo, error) {
	return f.CredentialsInfo(), f.err
}
func (f *fakeSession) BeginLogin(context.Context) (kite.LoginStart, error) {
	return kite.LoginStart{LoginURL: "https://kite.zerodha.com/connect/login?api_key=k&v=3", RedirectURL: kite.RedirectURL(7420), ListenerOK: true}, f.err
}
func (f *fakeSession) CompleteLogin(_ context.Context, p string) (kite.SessionState, error) {
	f.mu.Lock()
	f.completed = append(f.completed, p)
	f.mu.Unlock()
	return f.State(), f.err
}
func (f *fakeSession) CompleteForwardedLogin(_ context.Context, u string) (kite.SessionState, error) {
	f.mu.Lock()
	f.forwarded = append(f.forwarded, u)
	f.mu.Unlock()
	return f.State(), f.err
}
func (f *fakeSession) CancelLogin() kite.SessionState { return f.State() }
func (f *fakeSession) Logout(context.Context) (kite.LogoutResult, error) {
	return kite.LogoutResult{RemoteInvalidated: true}, f.err
}
func (f *fakeSession) AckRisk(context.Context) (kite.SessionState, error) { return f.State(), f.err }
func (f *fakeSession) Profile(context.Context) (kite.Profile, error)      { return f.profile, f.err }
func (f *fakeSession) Restore(context.Context) kite.SessionState          { return f.State() }
func (f *fakeSession) Run(ctx context.Context)                            { <-ctx.Done() }

type fakeSync struct {
	mu         sync.Mutex
	running    int
	maxRunning int           // highest number of concurrent Run calls seen
	exitDelay  time.Duration // slow exit after ctx is cancelled
	runs       int
	plan       kite.DataPlan
	refreshed  []orders.Scope
	refreshErr error
}

func (f *fakeSync) Run(ctx context.Context) {
	f.mu.Lock()
	f.running++
	f.runs++
	if f.running > f.maxRunning {
		f.maxRunning = f.running
	}
	delay := f.exitDelay
	f.mu.Unlock()
	<-ctx.Done()
	time.Sleep(delay)
	f.mu.Lock()
	f.running--
	f.mu.Unlock()
}
func (f *fakeSync) SetDataPlan(p kite.DataPlan) { f.mu.Lock(); defer f.mu.Unlock(); f.plan = p }
func (f *fakeSync) Refresh(_ context.Context, sc orders.Scope) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshed = append(f.refreshed, sc)
	return f.refreshErr
}
func (f *fakeSync) Book() orders.BookSnapshot           { return orders.BookSnapshot{AsOfMs: 1} }
func (f *fakeSync) Positions() orders.PositionsSnapshot { return orders.PositionsSnapshot{AsOfMs: 2} }
func (f *fakeSync) Holdings() orders.HoldingsSnapshot   { return orders.HoldingsSnapshot{AsOfMs: 3} }
func (f *fakeSync) snapshot() (int, int, kite.DataPlan) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running, f.runs, f.plan
}

type fakeInstruments struct {
	m map[uint32]instruments.Instrument
}

func (f *fakeInstruments) Search(q string, limit int) []instruments.Result {
	var out []instruments.Result
	for _, in := range f.m {
		if in.Tradingsymbol == q && len(out) < limit {
			out = append(out, instruments.Result{Instrument: in})
		}
	}
	return out
}
func (f *fakeInstruments) ByToken(t uint32) (instruments.Instrument, bool) {
	in, ok := f.m[t]
	return in, ok
}
func (f *fakeInstruments) ByID(id string) (instruments.Instrument, bool) {
	for _, in := range f.m {
		if in.ID() == id {
			return in, true
		}
	}
	return instruments.Instrument{}, false
}

type fakeCandles struct {
	have    []candles.Candle
	missing []candles.Range
}

func (f *fakeCandles) Read(context.Context, uint32, candles.Interval, time.Time, time.Time) ([]candles.Candle, error) {
	return f.have, nil
}
func (f *fakeCandles) Missing(context.Context, uint32, candles.Interval, time.Time, time.Time) ([]candles.Range, error) {
	return f.missing, nil
}

type fakeBackfill struct {
	mu   sync.Mutex
	jobs []backfill.Job
}

func (f *fakeBackfill) Submit(_ context.Context, j backfill.Job) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = append(f.jobs, j)
	return fmt.Sprintf("job-%d", len(f.jobs)), nil
}

type liveKey struct {
	c     ticker.Consumer
	owner string
}

// fakeLive mirrors feed.Feed's surface: subscriptions per (consumer, owner).
type fakeLive struct {
	mu    sync.Mutex
	subs  map[liveKey]map[uint32]bool
	unsub [][]uint32
	err   error
}

func (f *fakeLive) Subscribe(c ticker.Consumer, owner string, toks []uint32, _ ticker.Mode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	k := liveKey{c, owner}
	if f.subs[k] == nil {
		f.subs[k] = map[uint32]bool{}
	}
	for _, t := range toks {
		f.subs[k][t] = true
	}
	return nil
}
func (f *fakeLive) Unsubscribe(c ticker.Consumer, owner string, toks []uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unsub = append(f.unsub, toks)
	for _, t := range toks {
		delete(f.subs[liveKey{c, owner}], t)
	}
}
func (f *fakeLive) Release(c ticker.Consumer, owner string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.subs, liveKey{c, owner})
}

// has reports whether any owner of consumer c holds token t.
func (f *fakeLive) has(c ticker.Consumer, t uint32) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, set := range f.subs {
		if k.c == c && set[t] {
			return true
		}
	}
	return false
}

type svcEnv struct {
	svc      *Service
	session  *fakeSession
	sync     *fakeSync
	instr    *fakeInstruments
	candles  *fakeCandles
	backfill *fakeBackfill
	live     *fakeLive
	clock    *clock.Fake
	bus      *bus.Bus
	settings *settings.Settings
}

func ist(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, clock.IST)
}

func newSvcEnv(t *testing.T) *svcEnv {
	t.Helper()
	db := storetest.Open(t)
	b := bus.New(nil)
	t.Cleanup(b.Close)
	st, err := settings.Load(context.Background(), db.SettingsRepo(), b)
	if err != nil {
		t.Fatal(err)
	}
	e := &svcEnv{
		session:  &fakeSession{st: kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s1"}},
		sync:     &fakeSync{},
		instr:    &fakeInstruments{m: map[uint32]instruments.Instrument{}},
		candles:  &fakeCandles{},
		backfill: &fakeBackfill{},
		live:     &fakeLive{subs: map[liveKey]map[uint32]bool{}},
		clock:    clock.NewFake(ist(2026, 10, 12, 10, 0)),
		bus:      b,
		settings: st,
	}
	n := 0
	e.svc = New(Deps{
		Session: e.session, Orders: e.sync, Settings: st, Watchlists: store.NewWatchlistsRepo(db),
		Candles: e.candles, Backfill: e.backfill, Instruments: e.instr, Live: e.live,
		Clock: e.clock, Bus: b, Log: slog.Default(),
		NewID: func() string { n++; return fmt.Sprintf("wl-%d", n) },
	})
	return e
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

func refusedCode(t *testing.T, err error) string {
	t.Helper()
	r, ok := err.(*ErrRefused)
	if !ok {
		t.Fatalf("error %v (%T) is not *ErrRefused", err, err)
	}
	return r.Code
}
