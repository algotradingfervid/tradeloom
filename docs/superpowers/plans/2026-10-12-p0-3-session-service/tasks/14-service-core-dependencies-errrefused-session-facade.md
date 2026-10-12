> **P0-3 Session & Service Layer Implementation Plan — Task 14.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 14: `service` core — dependencies, `ErrRefused`, session façade, lifecycle supervisor

**Files:**
- Create: `internal/service/deps.go`, `internal/service/session.go`
- Test: `internal/service/env_test.go` (shared fakes for Tasks 14–20), `internal/service/session_test.go`

**Interfaces:**
- Consumes: Tasks 6–11 (through the `SessionAPI` interface, which `*kite.Session` satisfies). `orders.Scope` and the snapshot types (Task 12), `store.WatchlistsRepo` (Task 13), `storetest.Open` (Task 5), and from "Consumes": P0-1 `bus.New(nil)`, `bus.SubOpts[T]`, `bus.Unsubscribe`, `settings.Load`; P0-2 `candles.Store.Read/Missing` (`[]candles.Range`), `(*backfill.Backfill).Submit`, `(*instruments.Master).Search(q, limit) []instruments.Result` / `ByToken` / `ByID`, `(*feed.Feed).Subscribe(c, owner, tokens, mode)` / `Unsubscribe` / `Release`.
- Produces:
  ```go
  type SessionAPI interface { State() kite.SessionState; CredentialsInfo() kite.CredentialsInfo
      SaveCredentials(ctx context.Context, apiKey, apiSecret string) (kite.SessionState, error); RemoveCredentials(ctx context.Context) (kite.SessionState, error)
      SetRedirectPort(port int) (kite.CredentialsInfo, error); BeginLogin(ctx context.Context) (kite.LoginStart, error)
      CompleteLogin(ctx context.Context, pasted string) (kite.SessionState, error); CancelLogin() kite.SessionState
      Logout(ctx context.Context) (kite.LogoutResult, error); AckRisk(ctx context.Context) (kite.SessionState, error)
      Profile(ctx context.Context) (kite.Profile, error); Restore(ctx context.Context) kite.SessionState; Run(ctx context.Context) }
  type PortfolioSync interface { Run(ctx context.Context); SetDataPlan(p kite.DataPlan); Refresh(ctx context.Context, sc orders.Scope) error
      Book() orders.BookSnapshot; Positions() orders.PositionsSnapshot; Holdings() orders.HoldingsSnapshot }
  type CandleReader interface { Read(ctx, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Candle, error); Missing(…) ([]candles.Range, error) } // *sqlite.Store
  type BackfillSubmitter interface { Submit(ctx context.Context, j backfill.Job) (string, error) }                                                           // *backfill.Backfill
  type InstrumentFinder interface { Search(q string, limit int) []instruments.Result; ByToken(token uint32) (instruments.Instrument, bool); ByID(id string) (instruments.Instrument, bool) } // *instruments.Master
  type LiveSubscriber interface { Subscribe(c ticker.Consumer, owner string, tokens []uint32, m ticker.Mode) error; Unsubscribe(c ticker.Consumer, owner string, tokens []uint32); Release(c ticker.Consumer, owner string) } // *feed.Feed
  type Deps struct{ Session SessionAPI; Orders PortfolioSync; Settings *settings.Settings; Watchlists *store.WatchlistsRepo; Candles CandleReader
      Backfill BackfillSubmitter; Instruments InstrumentFinder; Live LiveSubscriber; Clock clock.Clock; Bus *bus.Bus; Log *slog.Logger; NewID func() string }
  func New(d Deps) *Service
  type ErrRefused struct{ Code, Stage, Reason string } // Error() == Reason
  type SessionStatus struct{ State, SessionID, UserID string; LoggedInAtMs, ExpiresAtMs int64; DataPlan string; Offline, RiskAckRequired bool; ErrorCode, ErrorMessage string; Credentials CredentialsInfo }
  type CredentialsInput struct{ APIKey, APISecret string }
  type CredentialsInfo = kite.CredentialsInfo; type LoginStart = kite.LoginStart; type LogoutResult = kite.LogoutResult; type Profile = kite.Profile
  // session methods as listed in "Produces (for P0-4 / P0-5)", plus:
  func (s *Service) Restore(ctx context.Context) SessionStatus
  func (s *Service) RunSession(ctx context.Context)
  func (s *Service) Run(ctx context.Context) // starts orders.Sync on ready, cancels it on any other state
  ```

- [ ] **Step 1: Write the shared test environment**

```go
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
	profile   kite.Profile
}

func (f *fakeSession) State() kite.SessionState { f.mu.Lock(); defer f.mu.Unlock(); return f.st }
func (f *fakeSession) setState(st kite.SessionState) { f.mu.Lock(); defer f.mu.Unlock(); f.st = st }
func (f *fakeSession) CredentialsInfo() kite.CredentialsInfo {
	return kite.CredentialsInfo{Configured: true, APIKeyMasked: "••••abcd", Port: 7420, RedirectURL: kite.RedirectURL(7420), SecretBackend: "memory"}
}
func (f *fakeSession) SaveCredentials(context.Context, string, string) (kite.SessionState, error) { return f.State(), f.err }
func (f *fakeSession) RemoveCredentials(context.Context) (kite.SessionState, error)               { return f.State(), f.err }
func (f *fakeSession) SetRedirectPort(int) (kite.CredentialsInfo, error)                          { return f.CredentialsInfo(), f.err }
func (f *fakeSession) BeginLogin(context.Context) (kite.LoginStart, error) {
	return kite.LoginStart{LoginURL: "https://kite.zerodha.com/connect/login?api_key=k&v=3", RedirectURL: kite.RedirectURL(7420), ListenerOK: true}, f.err
}
func (f *fakeSession) CompleteLogin(_ context.Context, p string) (kite.SessionState, error) {
	f.mu.Lock()
	f.completed = append(f.completed, p)
	f.mu.Unlock()
	return f.State(), f.err
}
func (f *fakeSession) CancelLogin() kite.SessionState                        { return f.State() }
func (f *fakeSession) Logout(context.Context) (kite.LogoutResult, error)     { return kite.LogoutResult{RemoteInvalidated: true}, f.err }
func (f *fakeSession) AckRisk(context.Context) (kite.SessionState, error)    { return f.State(), f.err }
func (f *fakeSession) Profile(context.Context) (kite.Profile, error)         { return f.profile, f.err }
func (f *fakeSession) Restore(context.Context) kite.SessionState             { return f.State() }
func (f *fakeSession) Run(ctx context.Context)                               { <-ctx.Done() }

type fakeSync struct {
	mu         sync.Mutex
	running    int
	runs       int
	plan       kite.DataPlan
	refreshed  []orders.Scope
	refreshErr error
}

func (f *fakeSync) Run(ctx context.Context) {
	f.mu.Lock()
	f.running++
	f.runs++
	f.mu.Unlock()
	<-ctx.Done()
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

type fakeInstruments struct{ m map[uint32]instruments.Instrument }

func (f *fakeInstruments) Search(q string, limit int) []instruments.Result {
	var out []instruments.Result
	for _, in := range f.m {
		if in.Tradingsymbol == q && len(out) < limit {
			out = append(out, instruments.Result{Instrument: in})
		}
	}
	return out
}
func (f *fakeInstruments) ByToken(t uint32) (instruments.Instrument, bool) { in, ok := f.m[t]; return in, ok }
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
	svc       *Service
	session   *fakeSession
	sync      *fakeSync
	instr     *fakeInstruments
	candles   *fakeCandles
	backfill  *fakeBackfill
	live      *fakeLive
	clock     *clock.Fake
	bus       *bus.Bus
	settings  *settings.Settings
}

func ist(y int, m time.Month, d, hh, mm int) time.Time { return time.Date(y, m, d, hh, mm, 0, 0, clock.IST) }

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
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/service/session_test.go
package service

import (
	"context"
	"errors"
	"testing"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite"
)

func TestAsRefused_MapsLoginError(t *testing.T) {
	err := asRefused("login", &kite.LoginError{Code: kite.CodeRequestTokenInvalid, Message: "Click again"})
	var r *ErrRefused
	if !errors.As(err, &r) || r.Code != kite.CodeRequestTokenInvalid || r.Stage != "login" || r.Reason != "Click again" {
		t.Fatalf("%#v", err)
	}
	if asRefused("x", nil) != nil {
		t.Fatal("nil must stay nil")
	}
	if got := asRefused("x", errors.New("boom")).(*ErrRefused); got.Code != "internal" {
		t.Fatalf("%#v", got)
	}
}

func TestSessionStatus_MergesCredentials(t *testing.T) {
	e := newSvcEnv(t)
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanNone, ExpiresAtMs: 42, RiskAckRequired: true})
	st := e.svc.SessionStatus(context.Background())
	if st.State != "ready" || st.DataPlan != "none" || st.ExpiresAtMs != 42 || !st.RiskAckRequired || st.Credentials.APIKeyMasked != "••••abcd" {
		t.Fatalf("%+v", st)
	}
}

func TestBeginLogin_ErrorIsRefused(t *testing.T) {
	e := newSvcEnv(t)
	e.session.err = &kite.LoginError{Code: kite.CodeNoCredentials, Message: "Set up first"}
	_, err := e.svc.BeginLogin(context.Background())
	if refusedCode(t, err) != kite.CodeNoCredentials {
		t.Fatal(err)
	}
}

// Review Focus 3: after expiry orders.Sync stops, so nothing polls Kite until the next login.
func TestRun_StopsSyncWhenSessionExpires(t *testing.T) {
	e := newSvcEnv(t)
	// Ready before Run starts: Run applies the current state right after subscribing, so no event can be missed.
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s1"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.svc.Run(ctx)
	eventually(t, "sync running", func() bool { r, _, p := e.sync.snapshot(); return r == 1 && p == kite.PlanConnect })
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanNone, SessionID: "s1"})
	eventually(t, "plan update", func() bool { _, _, p := e.sync.snapshot(); return p == kite.PlanNone })
	if _, runs, _ := e.sync.snapshot(); runs != 1 {
		t.Fatalf("sync restarted %d times for the same session", runs)
	}
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateExpired, ErrorCode: kite.CodeSessionExpired})
	eventually(t, "sync stopped", func() bool { r, _, _ := e.sync.snapshot(); return r == 0 })
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run 'TestAsRefused|TestSessionStatus|TestBeginLogin|TestRun_' -v`
Expected: FAIL to compile with `undefined: New`, `undefined: asRefused`.

- [ ] **Step 4: Write the implementation**

```go
// internal/service/deps.go
package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
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
)

type SessionAPI interface {
	State() kite.SessionState
	CredentialsInfo() kite.CredentialsInfo
	SaveCredentials(ctx context.Context, apiKey, apiSecret string) (kite.SessionState, error)
	RemoveCredentials(ctx context.Context) (kite.SessionState, error)
	SetRedirectPort(port int) (kite.CredentialsInfo, error)
	BeginLogin(ctx context.Context) (kite.LoginStart, error)
	CompleteLogin(ctx context.Context, pasted string) (kite.SessionState, error)
	CancelLogin() kite.SessionState
	Logout(ctx context.Context) (kite.LogoutResult, error)
	AckRisk(ctx context.Context) (kite.SessionState, error)
	Profile(ctx context.Context) (kite.Profile, error)
	Restore(ctx context.Context) kite.SessionState
	Run(ctx context.Context)
}

type PortfolioSync interface {
	Run(ctx context.Context)
	SetDataPlan(p kite.DataPlan)
	Refresh(ctx context.Context, sc orders.Scope) error
	Book() orders.BookSnapshot
	Positions() orders.PositionsSnapshot
	Holdings() orders.HoldingsSnapshot
}

// CandleReader is the read half of candles.Store (*sqlite.Store).
type CandleReader interface {
	Read(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Candle, error)
	Missing(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) ([]candles.Range, error)
}

// BackfillSubmitter is *backfill.Backfill.
type BackfillSubmitter interface {
	Submit(ctx context.Context, j backfill.Job) (string, error)
}

// InstrumentFinder is *instruments.Master.
type InstrumentFinder interface {
	Search(q string, limit int) []instruments.Result
	ByToken(token uint32) (instruments.Instrument, bool)
	ByID(id string) (instruments.Instrument, bool)
}

// LiveSubscriber is *feed.Feed: subscriptions are owned per (consumer, owner) so two consumers sharing a
// token never unsubscribe each other.
type LiveSubscriber interface {
	Subscribe(c ticker.Consumer, owner string, tokens []uint32, m ticker.Mode) error
	Unsubscribe(c ticker.Consumer, owner string, tokens []uint32)
	Release(c ticker.Consumer, owner string)
}

type Deps struct {
	Session     SessionAPI
	Orders      PortfolioSync
	Settings    *settings.Settings
	Watchlists  *store.WatchlistsRepo
	Candles     CandleReader
	Backfill    BackfillSubmitter
	Instruments InstrumentFinder
	Live        LiveSubscriber
	Clock       clock.Clock
	Bus         *bus.Bus
	Log         *slog.Logger
	NewID       func() string
}

// Service is the transport-neutral command façade (ARCH §2.2, AD-09). The Wails bindings and (P5) Telegram call only this.
type Service struct {
	d Deps

	wlMu sync.Mutex // serialises watchlist read-modify-write

	liveMu sync.Mutex
	live   map[ticker.Consumer]map[uint32]bool
}

func New(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Service{d: d, live: map[ticker.Consumer]map[uint32]bool{}}
}

func (s *Service) nowMs() int64 { return s.d.Clock.Now().UnixMilli() }

// ErrRefused is the one error type the UI sees; internal/app serialises it as {"code","stage","reason"} (§8.4).
type ErrRefused struct {
	Code   string
	Stage  string
	Reason string
}

func (e *ErrRefused) Error() string { return e.Reason }

func refused(stage, code, reason string) error {
	return &ErrRefused{Code: code, Stage: stage, Reason: reason}
}

func asRefused(stage string, err error) error {
	if err == nil {
		return nil
	}
	var r *ErrRefused
	if errors.As(err, &r) {
		return r
	}
	var le *kite.LoginError
	if errors.As(err, &le) {
		return &ErrRefused{Code: le.Code, Stage: stage, Reason: le.Message}
	}
	return &ErrRefused{Code: "internal", Stage: stage, Reason: err.Error()}
}
```

```go
// internal/service/session.go
package service

import (
	"context"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite"
	"tradeloom/internal/logging"
)

type (
	CredentialsInfo = kite.CredentialsInfo
	LoginStart      = kite.LoginStart
	LogoutResult    = kite.LogoutResult
	Profile         = kite.Profile
)

type CredentialsInput struct {
	APIKey    string `json:"apiKey"`
	APISecret string `json:"apiSecret"`
}

// SessionStatus is the payload of Session.Status() and of the "session.state" event.
type SessionStatus struct {
	State           string          `json:"state"`
	SessionID       string          `json:"sessionId"`
	UserID          string          `json:"userId"`
	LoggedInAtMs    int64           `json:"loggedInAtMs"`
	ExpiresAtMs     int64           `json:"expiresAtMs"`
	DataPlan        string          `json:"dataPlan"`
	Offline         bool            `json:"offline"`
	RiskAckRequired bool            `json:"riskAckRequired"`
	ErrorCode       string          `json:"errorCode"`
	ErrorMessage    string          `json:"errorMessage"`
	Credentials     CredentialsInfo `json:"credentials"`
}

func (s *Service) statusFrom(st kite.SessionState) SessionStatus {
	return SessionStatus{
		State: string(st.State), SessionID: st.SessionID, UserID: st.UserID, LoggedInAtMs: st.LoggedInAtMs,
		ExpiresAtMs: st.ExpiresAtMs, DataPlan: string(st.DataPlan), Offline: st.Offline, RiskAckRequired: st.RiskAckRequired,
		ErrorCode: st.ErrorCode, ErrorMessage: st.ErrorMessage, Credentials: s.d.Session.CredentialsInfo(),
	}
}

func (s *Service) SessionStatus(_ context.Context) SessionStatus {
	return s.statusFrom(s.d.Session.State())
}

func (s *Service) CredentialsInfo(_ context.Context) CredentialsInfo {
	return s.d.Session.CredentialsInfo()
}

func (s *Service) SaveCredentials(ctx context.Context, in CredentialsInput) (SessionStatus, error) {
	st, err := s.d.Session.SaveCredentials(ctx, in.APIKey, in.APISecret)
	return s.statusFrom(st), asRefused("credentials", err)
}

func (s *Service) RemoveCredentials(ctx context.Context) (SessionStatus, error) {
	st, err := s.d.Session.RemoveCredentials(ctx)
	return s.statusFrom(st), asRefused("credentials", err)
}

func (s *Service) SetRedirectPort(_ context.Context, port int) (CredentialsInfo, error) {
	info, err := s.d.Session.SetRedirectPort(port)
	return info, asRefused("credentials", err)
}

func (s *Service) BeginLogin(ctx context.Context) (LoginStart, error) {
	ls, err := s.d.Session.BeginLogin(ctx)
	return ls, asRefused("login", err)
}

func (s *Service) CompleteLogin(ctx context.Context, pasted string) (SessionStatus, error) {
	st, err := s.d.Session.CompleteLogin(ctx, pasted)
	return s.statusFrom(st), asRefused("login", err)
}

func (s *Service) CancelLogin(_ context.Context) SessionStatus {
	return s.statusFrom(s.d.Session.CancelLogin())
}

func (s *Service) Logout(ctx context.Context) (LogoutResult, error) {
	r, err := s.d.Session.Logout(ctx)
	return r, asRefused("logout", err)
}

func (s *Service) AckRiskDisclosure(ctx context.Context) (SessionStatus, error) {
	st, err := s.d.Session.AckRisk(ctx)
	return s.statusFrom(st), asRefused("risk", err)
}

func (s *Service) Profile(ctx context.Context) (Profile, error) {
	p, err := s.d.Session.Profile(ctx)
	return p, asRefused("profile", err)
}

func (s *Service) Restore(ctx context.Context) SessionStatus {
	return s.statusFrom(s.d.Session.Restore(ctx))
}

func (s *Service) RunSession(ctx context.Context) { s.d.Session.Run(ctx) }

// Run supervises session-scoped services: orders.Sync runs only while the session is ready.
func (s *Service) Run(ctx context.Context) {
	sub := bus.Subscribe(s.d.Bus, kite.TopicSessionState, bus.SubOpts[kite.SessionState]{Name: "service.supervisor", Buffer: 16})
	defer bus.Unsubscribe(s.d.Bus, sub)
	var (
		cancel  context.CancelFunc
		session string
	)
	stop := func() {
		if cancel != nil {
			cancel()
			cancel = nil
			session = ""
		}
	}
	apply := func(st kite.SessionState) {
		if st.State != kite.StateReady {
			stop()
			return
		}
		s.d.Orders.SetDataPlan(st.DataPlan)
		if cancel != nil && session == st.SessionID {
			return
		}
		stop()
		c, cn := context.WithCancel(ctx)
		cancel, session = cn, st.SessionID
		logging.Go("orders.sync", func() { s.d.Orders.Run(c) })
	}
	apply(s.d.Session.State())
	for {
		select {
		case <-ctx.Done():
			stop()
			return
		case st := <-sub.C():
			apply(st)
		}
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ -run 'TestAsRefused|TestSessionStatus|TestBeginLogin|TestRun_' -v`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/service/deps.go internal/service/session.go internal/service/env_test.go internal/service/session_test.go
git commit -m "feat(service): façade core, ErrRefused and session supervisor

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
