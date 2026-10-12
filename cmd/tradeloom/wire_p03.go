// cmd/tradeloom/wire_p03.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/options"

	"tradeloom/internal/app"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles/backfill"
	"tradeloom/internal/candles/builder"
	candlesql "tradeloom/internal/candles/sqlite"
	"tradeloom/internal/clock"
	"tradeloom/internal/instruments"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/feed"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/licences"
	"tradeloom/internal/logging"
	"tradeloom/internal/orders"
	"tradeloom/internal/secrets"
	"tradeloom/internal/service"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

// Compile-time wiring checks (r70 review → Task 23): the real *kite.Client is the session's token sink and
// numbers its tokens, so the Ruling 70 stale-TokenException guard is active in production.
var (
	_ kite.TokenSink                 = (*kite.Client)(nil)
	_ interface{ TokenGen() uint64 } = (*kite.Client)(nil)
	_ service.SessionAPI             = (*kite.Session)(nil)
)

// p03Platform is every P0-2 component, built once per process by buildPlatform on top of P0-1's Boot.
type p03Platform struct {
	Boot      *app.Boot
	Secrets   *service.SecretSwitch
	Clock     *clock.Real
	Relay     *kite.SessionErrorRelay
	Kite      *kite.Client
	Limiter   *ratelimit.Limiter
	Calendar  *calendar.Service
	Candles   *candlesql.Store
	Master    *instruments.Master
	Refresher *instruments.Refresher
	Backfill  *backfill.Backfill
	Feed      *feed.Feed
	Builder   *builder.Builder
	Log       *slog.Logger

	// Set by wireP03.
	Session *kite.Session
	Service *service.Service

	cancel   context.CancelFunc // platform context (limiter, backfill, refresher, builder); set by Start
	wg       sync.WaitGroup     // the goroutines Start launches; Stop waits for them before closing the candle store
	stopOnce sync.Once
}

// buildPlatform wires P0-2 in ARCHITECTURE §3.2 order: clock → limiter → allow-list transport → Kite client
// (session errors through the relay) → calendar + persisted overrides → candle store → instrument master
// → backfill → feed → candle builder → secrets backend. Nothing runs yet: Start does. The instrument
// master is loaded by wireP03 once Service.LoadCalendar has applied the calendar (Ruling 77).
func buildPlatform(ctx context.Context, boot *app.Boot) (*p03Platform, error) {
	log := boot.Log.Logger // redacting (logging.New wraps every record in RedactHandler)
	p := &p03Platform{Boot: boot, Log: log}
	p.Clock = clock.New(clock.Options{Bus: boot.Bus, Log: log.With("component", "clock")})
	p.Limiter = ratelimit.New(p.Clock, ratelimit.Options{Bus: boot.Bus, Log: log.With("component", "ratelimit")})
	// NFR-04: nil next = http.DefaultTransport behind the allow-list (cmd never imports net/http, Ruling 79).
	transport := kite.NewAllowListTransport(nil, kite.AllowedHosts)
	p.Relay = &kite.SessionErrorRelay{}
	p.Kite = kite.New(kite.Options{
		APIKey:         settings.Get(boot.Settings, kite.SettingAPIKey),
		Limiter:        p.Limiter,
		Clock:          p.Clock,
		DateSink:       p.Clock.ObserveHTTPDate,
		OnSessionError: p.Relay.Handle,
		Transport:      transport,
		Log:            log.With("component", "kite"),
	})
	cal, err := calendar.NewDefault()
	if err != nil {
		return nil, fmt.Errorf("calendar: %w", err)
	}
	rows, err := boot.DB.CalendarOverrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar overrides: %w", err)
	}
	overrides := make([]calendar.Entry, 0, len(rows))
	for _, r := range rows {
		e, err := calendar.FromRecord(calendar.Record{Exchange: r.Exchange, Date: r.Date, Type: r.Type, Phases: r.Phases, Source: r.Source})
		if err != nil {
			log.Warn("calendar.override_skipped", "component", "calendar", "exchange", r.Exchange, "date", r.Date, "err", err)
			continue
		}
		overrides = append(overrides, e)
	}
	cal.SetOverrides(overrides)
	p.Calendar = cal
	cstore, err := candlesql.Open(ctx, boot.Layout.CandlesDB, candlesql.Options{Clock: p.Clock, Calendar: cal})
	if err != nil {
		return nil, fmt.Errorf("candle store: %w", err)
	}
	p.Candles = cstore
	p.Master = instruments.NewMaster(p.Clock)
	p.Refresher = instruments.NewRefresher(instruments.Options{
		Client: p.Kite, Master: p.Master, DB: boot.DB, Calendar: cal, Clock: p.Clock, Bus: boot.Bus,
		DataDir: boot.Layout.Root, Log: log.With("component", "instruments"),
	})
	p.Backfill = backfill.New(backfill.Options{Store: cstore, Jobs: cstore, Fetcher: p.Kite, Clock: p.Clock, Bus: boot.Bus, Log: log.With("component", "backfill")})
	p.Feed = feed.New(feed.Options{
		Ticker: ticker.Options{Clock: p.Clock, Bus: boot.Bus, Log: log.With("component", "ticker"), ClockRef: kite.NiftyIndexToken, Observer: p.Clock},
		Log:    log.With("component", "feed"),
	})
	p.Builder = builder.New(builder.Options{Clock: p.Clock, Calendar: cal, Store: cstore, Bus: boot.Bus, ExchangeOf: p.Master.ExchangeOf, Log: log.With("component", "builder")})
	p.Secrets = secretSwitch(boot.Secrets)
	return p, nil
}

// secretSwitch maps P0-1's detection to Task 27's runtime backend (FR-ACCT-013): the keyring is usable at
// once; an existing secrets.enc starts locked until Session.Unlock opens it; "choose" starts as NewNone
// (refuses writes with secrets.ErrStorageRefused) until the user picks the encrypted file or no storage.
// Ruling 72 (b): nothing here (nor any other start-up step) sets the switch's leftover-file flag, so a
// start with both secrets.enc and keyring keys never deletes secrets.enc.
func secretSwitch(d secrets.Detection) *service.SecretSwitch {
	switch d.Mode {
	case secrets.ModeKeyring:
		return service.NewSecretSwitch(secrets.NewKeyring(), false)
	case secrets.ModeFile:
		return service.NewSecretSwitch(secrets.NewNone(), true)
	default:
		return service.NewSecretSwitch(secrets.NewNone(), false)
	}
}

// Start runs the process-wide P0-2 goroutines (App.Deps.StartPlatform, called from App.Startup).
func (p *p03Platform) Start(ctx context.Context) error {
	pctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.goPlatform("ratelimit", func() { p.Limiter.Run(pctx) })
	if err := p.Backfill.Start(pctx); err != nil {
		cancel() // Stop still waits for the limiter goroutine
		return fmt.Errorf("backfill: %w", err)
	}
	p.goPlatform("instruments.refresher", func() { p.Refresher.Run(pctx) })
	p.goPlatform("candles.builder", func() { p.Builder.Run(pctx, p.Feed.Ticks()) })
	return nil
}

func (p *p03Platform) goPlatform(name string, fn func()) {
	p.wg.Add(1)
	logging.Go(name, func() {
		defer p.wg.Done()
		fn()
	})
}

// Stop is ARCHITECTURE §3.3: stop the feed, cancel the platform context, wait for the platform goroutines
// and let the backfill drain, close the candle store. boot.Shutdown (bus, app.db, exit marker, lock, log)
// runs afterwards in run(). Stop is idempotent: a restore closes the platform early (CloseForRestore) and
// App.Shutdown calls it again.
func (p *p03Platform) Stop(context.Context) {
	p.stopOnce.Do(func() {
		p.Feed.Stop()
		if p.cancel != nil {
			p.cancel()
		}
		p.wg.Wait()
		p.Backfill.WaitStopped()
		if err := p.Candles.Close(); err != nil {
			p.Log.Warn("candles.close_failed", "component", "candles", "err", err)
		}
	})
}

// stopAll is App.Deps.StopPlatform. App.Shutdown has cancelled the App context; this closes the login
// listener and every live chart (Ruling 66), waits for the service's own goroutines (a running backup or
// support-bundle job, event sources, market clock; Ruling 74 (9)) and then stops the platform, all before
// run() lets boot.Shutdown close app.db. Idempotent.
func (p *p03Platform) stopAll(ctx context.Context) {
	if p.Session != nil {
		p.Session.Close()
	}
	if p.Service != nil {
		if n := p.Service.CloseAllLive(ctx); n > 0 {
			p.Log.Info("live charts closed at shutdown", "component", "app", "count", n)
		}
		p.Service.Wait()
	}
	p.Stop(ctx)
}

// CloseForRestore stops the platform and closes app.db so store.Restore can replace the data directory
// (service.Deps.CloseForRestore, Task 30). The process lock (Boot.Lock), the bus and the event bridge stay
// up so the restore job's StageRestart still reaches the UI (Rulings 74, 75); boot.Shutdown's later
// DB.Close is a no-op (store.DB.Close is idempotent).
func (p *p03Platform) CloseForRestore() error {
	p.Stop(context.Background())
	return p.Boot.DB.Close()
}

// wireP03 builds this plan's components on the platform and returns the Wails App.
func wireP03(p *p03Platform) *app.App {
	boot := p.Boot
	newID := func() string { return uuid.Must(uuid.NewV7()).String() }
	sess := kite.NewSession(kite.SessionDeps{
		Settings: boot.Settings, Secrets: p.Secrets, Repo: store.NewSessionsRepo(boot.DB), Clock: p.Clock, Bus: boot.Bus,
		Auth:     kite.NewAuth(nil, ""), // nil: 10 s client on the NFR-04 allow-list transport
		Profiles: p.Kite, Tokens: p.Kite, Limiter: p.Limiter, Feed: p.Feed,
		Probe:    kite.NewCapabilityProbe(p.Kite, p.Clock, nil, p.Log.With("component", "kite.probe")),
		Log:      p.Log.With("component", "kite.session"),
		NewID:    newID,
	})
	p.Relay.Set(sess.HandleTokenError)
	portfolio := orders.NewSync(orders.SyncDeps{Read: p.Kite, Cache: store.NewOrdersCacheRepo(boot.DB), Bus: boot.Bus, Clock: p.Clock,
		Live: p.Feed, LiveNotRunning: feed.ErrNotRunning, Log: p.Log.With("component", "orders.sync")})
	svc := service.New(service.Deps{
		// Task 14
		Session: sess, Orders: portfolio, Settings: boot.Settings, Watchlists: store.NewWatchlistsRepo(boot.DB),
		Candles: p.Candles, Backfill: p.Backfill, Instruments: p.Master, Live: p.Feed,
		Clock: p.Clock, Bus: boot.Bus, Log: p.Log.With("component", "service"), NewID: newID,
		// Task 16
		Calendar: p.Calendar, Historical: p.Kite, CandleWriter: p.Candles, Tracker: p.Builder,
		// Task 27: the same switch as SessionDeps.Secrets
		Secrets: p.Secrets, SecretsFile: boot.Layout.SecretsFile, Detection: boot.Secrets,
		KeyringProbe: func() error { return secrets.ProbeKeyringWithin(secrets.ProbeTimeout) }, NewKeyring: secrets.NewKeyring,
		// Task 28
		Shortcuts: store.NewShortcutsRepo(boot.DB), UiState: store.NewWindowStateRepo(boot.DB),
		// Task 29
		Quotes: p.Kite,
		// Task 30
		DB: boot.DB, Layout: boot.Layout, AppVersion: app.Version, CandleAdmin: p.Candles, Refresher: p.Refresher,
		ImportedAt: p.Master.ImportedAt, CloseForRestore: p.CloseForRestore,
		// Task 32
		CrashReports: boot.CrashReports,
		// Task 33
		FeedCtl: p.Feed, Licences: licences.Notices,
	})
	p.Session, p.Service = sess, svc
	// Ruling 77: saved overrides and calendar.covers_until are applied before anything reads the calendar
	// (instrument master, backfill, market clock, supervisor: all start later in App.Startup).
	ctx := context.Background()
	if err := svc.LoadCalendar(ctx); err != nil {
		p.Log.Warn("calendar.load_failed", "component", "calendar", "err", err)
	}
	if err := p.Refresher.Load(ctx); err != nil {
		p.Log.Warn("instruments.load_failed", "component", "instruments", "err", err) // the daily refresh retries (FR-WATCH-008 AC4)
	}
	return app.New(app.Deps{
		Boot: boot, Service: svc, Log: p.Log.With("component", "app"), StartPlatform: p.Start, StopPlatform: p.stopAll,
	})
}

// closeLiveOnReload chains closeAll before the options' OnDomReady: Wails calls OnDomReady on every page
// load, so a frontend reload drops the live chart subscriptions the old page opened (Ruling 66). The
// first load has none.
func closeLiveOnReload(o *options.App, closeAll func(ctx context.Context) int) {
	prev := o.OnDomReady
	o.OnDomReady = func(ctx context.Context) {
		closeAll(ctx)
		if prev != nil {
			prev(ctx)
		}
	}
}
