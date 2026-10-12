// internal/service/deps.go
package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/candles/backfill"
	"tradeloom/internal/clock"
	"tradeloom/internal/instruments"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/logging"
	"tradeloom/internal/orders"
	"tradeloom/internal/secrets"
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

// HistoricalFetcher is *kite.Client (kite.HistoricalPicker): the manual refresh fetches one window at Interactive priority.
type HistoricalFetcher interface {
	HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (kite.HistoricalRequest, bool)) (kite.HistoricalRequest, []kite.HistoricalData, bool, error)
}

// CandleUpserter is the covered-write half of candles.Store (*sqlite.Store): the manual refresh records
// coverage for the window it fetched, up to the first candle that has not ended (Ruling 66).
type CandleUpserter interface {
	UpsertCovered(ctx context.Context, token uint32, iv candles.Interval, window candles.Range, cs []candles.Candle, src candles.Source) error
}

// CandleTracker is *builder.Builder: forming/closed candles are built only for tracked (token, interval) pairs.
type CandleTracker interface {
	Track(token uint32, ivs ...candles.Interval)
	Untrack(token uint32, ivs ...candles.Interval)
}

// CandleAdmin is the maintenance half of *sqlite.Store (FR-DATA-002) (Task 30).
type CandleAdmin interface {
	Stats(ctx context.Context) (candles.Stats, error)
	Clear(ctx context.Context, confirmRuns []string) error
	OpenJobs(ctx context.Context) ([]candles.Job, error)
}

// InstrumentRefresher is *instruments.Refresher (Task 30): Refresh for the manual refresh, Trigger after
// each new login (P0-2 Ruling 49; non-blocking, coalescing).
type InstrumentRefresher interface {
	Refresh(ctx context.Context) error
	Trigger()
}

// QuoteReader is *kite.Client's quote half (REST fallback for tokens the feed cannot carry; Task 29).
type QuoteReader interface {
	LTP(ctx context.Context, ids ...string) (kite.QuoteLTP, error)
	OHLC(ctx context.Context, ids ...string) (kite.QuoteOHLC, error)
}

// liveChart is one open chart subscription (OpenLive).
type liveChart struct {
	token uint32
	iv    candles.Interval
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

	Shortcuts *store.ShortcutsRepo   // user rebinds (Task 28)
	UiState   *store.WindowStateRepo // window_state key/value (Task 28)

	// Task 16 (chart data)
	Calendar     *calendar.Service // trading-day arithmetic, market phase (Task 21), calendar settings (Task 31)
	Historical   HistoricalFetcher // *kite.Client
	CandleWriter CandleUpserter    // *sqlite.Store
	Tracker      CandleTracker     // *builder.Builder
	// Task 30 (data façade)
	DB              *store.DB           // app.db (backups, calendar overrides, support files)
	Layout          store.Layout        // data directory layout
	AppVersion      string              // app.Version
	CandleAdmin     CandleAdmin         // candle stats / clear
	Refresher       InstrumentRefresher // manual instrument refresh; Trigger after login
	ImportedAt      func() time.Time    // Master.ImportedAt
	CloseForRestore func() error        // stops the platform and closes app.db before store.Restore
	supportedSchema func() int          // store.SupportedSchema; tests override it

	// Task 29 (quotes)
	Quotes QuoteReader // *kite.Client

	// Secret storage (Task 27, FR-ACCT-013).
	Secrets      *SecretSwitch        // the Session's secrets.Store; swapped by setup, unlock and migration
	SecretsFile  string               // Layout.SecretsFile
	Detection    secrets.Detection    // Boot.Secrets
	KeyringProbe func() error         // secrets.ProbeKeyringWithin(secrets.ProbeTimeout); nil = available
	NewKeyring   func() secrets.Store // secrets.NewKeyring; tests inject a memory store
}

// Service is the transport-neutral command façade (ARCH §2.2, AD-09). The Wails bindings and (P5) Telegram call only this.
type Service struct {
	d Deps

	wlMu sync.Mutex // serialises watchlist read-modify-write

	pf          portfolioGate // RefreshPortfolio single-flight and minimum interval (Ruling 65)
	shortcutsMu sync.Mutex    // serialises Rebind / ResetShortcuts (conflict check + save)

	liveMu sync.Mutex
	live   map[ticker.Consumer]map[uint32]bool

	chartMu    sync.Mutex
	liveCharts map[string]liveChart // subscription id → (token, interval)

	// Task 21 (events): initialised lazily in events.go so New is untouched.
	quoteMu    sync.Mutex
	lastQuote  map[uint32]TickDTO // last quote per token (tick.batch, live or polled); Task 29 persists it
	quoteDirty bool

	feedMu   sync.Mutex
	lastFeed ticker.FeedState // last feed.state (System.Status); zero State reads as "stopped"

	// Task 30: the single job slot (backup, restore, support bundle).
	jobMu      sync.Mutex
	jobRunning string // kind of the running job, "" when idle
	jobSeq     int

	// Task 29 (quotes): the maps are initialised lazily in quotes.go so New is untouched.
	quotesOnce   sync.Once                                // loads the persisted quote cache
	watchMu      sync.Mutex                               // serialises set changes with their feed calls (WatchQuotes, linger)
	pollMu       sync.Mutex                               // guards polled, deferred, pending, pollOff, closeSession, closes, quitQuotes
	polled       map[ticker.Consumer]map[uint32]bool      // tokens on the REST path, per consumer
	deferred     map[ticker.Consumer]map[uint32]bool      // subscribed while the feed was stopped; polled until feed.state is live
	pending      map[ticker.Consumer]map[uint32]time.Time // removed tokens awaiting UnsubscribeLinger (their deadline)
	pollOff      string                                   // session id whose quote polling Kite refused (no data permission)
	closeSession string                                   // session id the closes belong to
	closes       map[uint32]float64                       // previous close per token, fetched once per session (OHLC)
	quitQuotes   chan struct{}                            // closed when RunQuotes exits; ends pending linger goroutines
	quitOnce     sync.Once
}

// New panics when a dependency the core cannot run without is missing (a wiring bug, caught at start-up).
func New(d Deps) *Service {
	switch {
	case d.Session == nil:
		panic("service.New: Deps.Session is nil")
	case d.Orders == nil:
		panic("service.New: Deps.Orders is nil")
	case d.Bus == nil:
		panic("service.New: Deps.Bus is nil")
	case d.Clock == nil:
		panic("service.New: Deps.Clock is nil")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Service{d: d, live: map[ticker.Consumer]map[uint32]bool{}, liveCharts: map[string]liveChart{}}
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

// asRefused maps any error to *ErrRefused. A *kite.LoginError keeps its code unchanged (e.g. secret_store
// is never folded into no_credentials, Ruling 52). Any other error becomes "internal". Every Reason passes
// through the redactor so a registered secret can never reach the UI (Ruling 61).
func asRefused(stage string, err error) error {
	if err == nil {
		return nil
	}
	var r *ErrRefused
	if errors.As(err, &r) {
		return &ErrRefused{Code: r.Code, Stage: r.Stage, Reason: logging.Default.RedactString(r.Reason)}
	}
	var le *kite.LoginError
	if errors.As(err, &le) {
		return &ErrRefused{Code: le.Code, Stage: stage, Reason: logging.Default.RedactString(le.Message)}
	}
	return &ErrRefused{Code: "internal", Stage: stage, Reason: logging.Default.RedactString(err.Error())}
}

// refuse is asRefused for façade methods. On the internal path it logs the original error at Warn
// through the injected (redacting) logger.
func (s *Service) refuse(stage string, err error) error {
	out := asRefused(stage, err)
	if r, ok := out.(*ErrRefused); ok && r.Code == "internal" {
		var known *ErrRefused
		if !errors.As(err, &known) {
			s.d.Log.Warn("service.internal_error", "stage", stage, "err", err)
		}
	}
	return out
}
