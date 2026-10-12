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

// CandleUpserter is the write half of candles.Store (*sqlite.Store).
type CandleUpserter interface {
	Upsert(ctx context.Context, token uint32, iv candles.Interval, cs []candles.Candle, src candles.Source) error
}

// CandleTracker is *builder.Builder: forming/closed candles are built only for tracked (token, interval) pairs.
type CandleTracker interface {
	Track(token uint32, ivs ...candles.Interval)
	Untrack(token uint32, ivs ...candles.Interval)
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
