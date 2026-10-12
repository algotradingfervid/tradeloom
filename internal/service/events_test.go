package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/candles/backfill"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/orders"
)

type emitted struct {
	name    string
	payload any
}

func startEvents(t *testing.T, e *svcEnv) chan emitted {
	t.Helper()
	ch := make(chan emitted, 256)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	for _, src := range e.svc.EventSources() {
		src.Start(ctx, func(n string, p any) { ch <- emitted{n, p} })
	}
	return ch
}

func nextEvent(t *testing.T, ch chan emitted, name string) emitted {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.name == name {
				return ev
			}
		case <-deadline:
			t.Fatalf("no %s event", name)
		}
	}
}

// advanceUntilEvent steps the fake clock by CoalesceWindow until the named event arrives (coalescing
// timers are armed asynchronously when a source receives its first value).
func advanceUntilEvent(t *testing.T, e *svcEnv, ch chan emitted, name string) emitted {
	t.Helper()
	for i := 0; i < 50; i++ {
		e.clock.Advance(CoalesceWindow)
		select {
		case ev := <-ch:
			if ev.name == name {
				return ev
			}
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatalf("no %s event after advancing the clock", name)
	return emitted{}
}

func TestEventSources_MatchCatalogue(t *testing.T) {
	e := newSvcEnv(t)
	cat := EventCatalogue()
	srcs := e.svc.EventSources()
	if len(cat) != len(srcs) {
		t.Fatalf("catalogue %d vs sources %d", len(cat), len(srcs))
	}
	for i := range cat {
		if cat[i].Name != srcs[i].Name {
			t.Fatalf("entry %d: catalogue %s, source %s", i, cat[i].Name, srcs[i].Name)
		}
	}
	want := []string{"session.state", "feed.state", "tick.batch", "candle.forming", "candle.closed", "clock.offset", "backfill.progress", "limits.usage",
		"orders.changed", "positions.changed", "holdings.changed", "order.update", "watchlists.changed", "settings.changed", "market.status", "job.progress", "app.navigate"}
	if len(cat) != len(want) {
		t.Fatalf("catalogue has %d entries, want %d", len(cat), len(want))
	}
	for i, w := range want {
		if cat[i].Name != w {
			t.Fatalf("entry %d is %s, want %s", i, cat[i].Name, w)
		}
	}
	for _, spec := range cat {
		switch spec.Name {
		case EvSessionState, EvCandleClosed, EvWatchlistsChanged, EvSettingsChanged, EvOrderUpdate, EvMarketStatus, EvJobProgress, EvAppNavigate:
			if !spec.Immediate {
				t.Fatalf("%s must be immediate", spec.Name)
			}
		default:
			if spec.Immediate {
				t.Fatalf("%s must be coalesced", spec.Name)
			}
		}
	}
}

// Rulings 59/61: kite.Session publishes outside its lock, so the bridge forwards Session.State() (the
// authoritative state), not the event payload. The fake's state is set before each publish.
func TestForwardNow_SessionStateEveryTransition(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	e.session.setState(kite.SessionState{State: kite.StateLoggingIn})
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateLoggingIn})
	first := nextEvent(t, ch, EvSessionState).payload.(SessionStatus)
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect})
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect})
	second := nextEvent(t, ch, EvSessionState).payload.(SessionStatus)
	if first.State != "logging_in" || second.State != "ready" || second.Credentials.RedirectURL == "" {
		t.Fatalf("%+v / %+v", first, second)
	}
}

// A stale "ready" delivered after a logout must not reach the UI as "ready".
func TestForwardNow_SessionStateUsesAuthoritativeState(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	e.session.setState(kite.SessionState{State: kite.StateLoggedOut})
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateReady, SessionID: "old", DataPlan: kite.PlanConnect})
	if got := nextEvent(t, ch, EvSessionState).payload.(SessionStatus); got.State != "logged_out" || got.SessionID != "" {
		t.Fatalf("stale payload forwarded: %+v", got)
	}
}

func TestForwardLatest_CoalescesWithin100ms(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	for i := int64(1); i <= 3; i++ {
		bus.Publish(e.bus, orders.TopicBook, orders.BookSnapshot{AsOfMs: i})
	}
	time.Sleep(50 * time.Millisecond)
	select {
	case ev := <-ch:
		t.Fatalf("emitted before the window closed: %+v", ev)
	default:
	}
	e.clock.Advance(CoalesceWindow)
	ev := nextEvent(t, ch, EvOrdersChanged)
	if ev.payload.(orders.BookSnapshot).AsOfMs != 3 {
		t.Fatalf("payload %+v", ev.payload)
	}
	time.Sleep(30 * time.Millisecond)
	select {
	case ev := <-ch:
		t.Fatalf("second emit %+v", ev)
	default:
	}
}

// Live WebSocket ticks and REST-polled quotes share one tick.batch and one last-quote cache.
func TestForwardTicks_MergesLiveAndPolledAndCaches(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	bus.Publish(e.bus, ticker.TopicTicks, []ticker.Tick{{Token: 1, LTP: 100}})
	bus.Publish(e.bus, ticker.TopicTicks, []ticker.Tick{{Token: 2, LTP: 200}, {Token: 1, LTP: 101}})
	bus.Publish(e.bus, TopicPolledTicks, []TickDTO{{Token: 3, LTP: 300, Close: 290, TsMs: 7}})
	time.Sleep(50 * time.Millisecond)
	got := advanceUntilEvent(t, e, ch, EvTickBatch).payload.([]TickDTO)
	if len(got) != 3 || got[0].Token != 1 || got[0].LTP != 101 || got[1].LTP != 200 || got[2].Token != 3 || !got[2].Polled || got[0].Polled {
		t.Fatalf("%+v", got)
	}
	last := e.svc.LastQuotes([]uint32{3, 1, 9})
	if len(last) != 2 || last[0].Token != 3 || last[0].LTP != 300 || last[1].Token != 1 || last[1].LTP != 101 {
		t.Fatalf("cache %+v", last)
	}
}

func TestForwardKeyed_BackfillProgressPerJob(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	bus.Publish(e.bus, backfill.TopicProgress, backfill.Progress{JobID: "a"})
	bus.Publish(e.bus, backfill.TopicProgress, backfill.Progress{JobID: "b"})
	bus.Publish(e.bus, backfill.TopicProgress, backfill.Progress{JobID: "a"})
	time.Sleep(50 * time.Millisecond)
	if got := advanceUntilEvent(t, e, ch, EvBackfillProgress).payload.([]BackfillProgressDTO); len(got) != 2 {
		t.Fatalf("%+v", got)
	}
}

// FR-ORD-014: order updates reach the UI unchanged and at once; the feed state is remembered for System.Status.
func TestForwardNow_OrderUpdateAndFeedState(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	if e.svc.FeedState().State != "stopped" {
		t.Fatalf("initial feed state %+v", e.svc.FeedState())
	}
	bus.Publish(e.bus, kite.TopicOrderUpdate, kite.Order{OrderID: "230", Status: "COMPLETE"})
	if o := nextEvent(t, ch, EvOrderUpdate).payload.(kite.Order); o.OrderID != "230" {
		t.Fatalf("%+v", o)
	}
	bus.Publish(e.bus, ticker.TopicFeed, ticker.FeedState{State: "live", Tokens: 3})
	time.Sleep(20 * time.Millisecond)
	if fs := advanceUntilEvent(t, e, ch, EvFeedState).payload.(FeedStateDTO); fs.State != "live" || fs.Tokens != 3 {
		t.Fatalf("%+v", fs)
	}
	if e.svc.FeedState().Tokens != 3 {
		t.Fatalf("feed state not remembered: %+v", e.svc.FeedState())
	}
	e.sync.mu.Lock()
	defer e.sync.mu.Unlock()
	if len(e.sync.refreshed) != 0 {
		t.Fatalf("refresh without drops: %v", e.sync.refreshed)
	}
}

// FR-SHELL-005: the market phase is published at start and again exactly at the 15:30 boundary.
func TestMarketStatus_PublishesAtStartAndAtPhaseBoundary(t *testing.T) {
	e := newSvcEnv(t)
	e.svc.d.Calendar = mustCalendar(t)
	e.clock.Set(ist(2026, 10, 12, 15, 29))
	ch := startEvents(t, e)
	first := nextEvent(t, ch, EvMarketStatus).payload.(MarketStatus)
	if first.Exchange != "NSE" || first.Phase != "continuous" || first.DayType != "trading" || first.Label != "Open" || first.AtMs != ist(2026, 10, 12, 15, 29).UnixMilli() {
		t.Fatalf("%+v", first)
	}
	if got := e.svc.MarketStatus(); got.Phase != "continuous" {
		t.Fatalf("on demand %+v", got)
	}
	e.clock.Advance(time.Minute)
	second := nextEvent(t, ch, EvMarketStatus).payload.(MarketStatus)
	if second.Phase != "closing" || second.Label != "Closing" || second.AtMs != ist(2026, 10, 12, 15, 30).UnixMilli() {
		t.Fatalf("%+v", second)
	}
	e.clock.Advance(30 * time.Second) // still closing: no event
	select {
	case ev := <-ch:
		if ev.name == EvMarketStatus {
			t.Fatalf("unexpected %+v", ev)
		}
	case <-time.After(30 * time.Millisecond):
	}
}

func TestNavigate_PublishesRoutesOnly(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	e.svc.Navigate("settings")
	e.svc.Navigate("//x")     // protocol-relative: never a route (settings.ValidateRoute)
	e.svc.Navigate("/a/../b") // dot-dot segment
	e.svc.Navigate("/settings/data")
	if nav := nextEvent(t, ch, EvAppNavigate).payload.(Navigate); nav.Route != "/settings/data" {
		t.Fatalf("%+v", nav)
	}
}

// Every bus receive checks ok: a closed bus ends the sources instead of emitting zero values in a loop.
func TestEventSources_ClosedBusEmitsNothing(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	e.bus.Close()
	for i := 0; i < 3; i++ {
		e.clock.Advance(CoalesceWindow)
	}
	select {
	case ev := <-ch:
		t.Fatalf("emitted after the bus closed: %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

// startSource starts only the named source with its own emit.
func startSource(t *testing.T, e *svcEnv, name string, emit Emit) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	for _, src := range e.svc.EventSources() {
		if src.Name == name {
			src.Start(ctx, emit)
			return cancel
		}
	}
	t.Fatalf("no source %s", name)
	return nil
}

// Ruling 67: no payload carries time.Time or time.Duration. kite.Order is forwarded unchanged; its
// models.Time fields marshal as strings (json.Marshaler), so the walk stops at marshalers.
func TestEventPayloads_NoTimeTypes(t *testing.T) {
	timeT, durT := reflect.TypeOf(time.Time{}), reflect.TypeOf(time.Duration(0))
	marshaler := reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	seen := map[reflect.Type]bool{}
	var walk func(ty reflect.Type, path string)
	walk = func(ty reflect.Type, path string) {
		if ty == timeT || ty == durT {
			t.Errorf("%s is %v", path, ty)
			return
		}
		if seen[ty] || ty.Implements(marshaler) || reflect.PointerTo(ty).Implements(marshaler) {
			return
		}
		seen[ty] = true
		switch ty.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.Pointer:
			walk(ty.Elem(), path+"[]")
		case reflect.Struct:
			for i := 0; i < ty.NumField(); i++ {
				if f := ty.Field(i); f.IsExported() && f.Tag.Get("json") != "-" {
					walk(f.Type, path+"."+f.Name)
				}
			}
		}
	}
	for _, spec := range EventCatalogue() {
		walk(spec.Payload, spec.Name)
	}
}

func TestPayloadDTOs_UseUnixMillis(t *testing.T) {
	ts := ist(2026, 10, 12, 10, 5)
	fs := FeedStateDTOFrom(ticker.FeedState{State: "reconnecting", Attempt: 2, NextRetry: ts, Connections: 1, Tokens: 4})
	if fs.State != "reconnecting" || fs.Attempt != 2 || fs.NextRetryMs != ts.UnixMilli() || fs.LastTickMs != 0 || fs.Connections != 1 || fs.Tokens != 4 {
		t.Fatalf("feed %+v", fs)
	}
	off := OffsetDTOFrom(clock.Offset{Millis: -12, Source: clock.SourceNone, Samples: 3, Warn: true, UpdatedAt: ts})
	if off.Millis != -12 || off.Source != string(clock.SourceNone) || off.Samples != 3 || !off.Warn || off.UpdatedAtMs != ts.UnixMilli() {
		t.Fatalf("offset %+v", off)
	}
	u := UsageDTOFrom(ratelimit.Usage{At: ts, Paused: true, PauseReason: "expired", Classes: []ratelimit.ClassUsage{{Class: "quote", RatePerSec: 1, HTTP429: 2, BackoffUntil: ts}, {Class: "hist"}}})
	if u.AtMs != ts.UnixMilli() || !u.Paused || u.PauseReason != "expired" || len(u.Classes) != 2 || u.Classes[0].BackoffUntilMs != ts.UnixMilli() ||
		u.Classes[0].HTTP429 != 2 || u.Classes[0].RatePerSec != 1 || u.Classes[1].BackoffUntilMs != 0 {
		t.Fatalf("usage %+v", u)
	}
	p := BackfillProgressDTOFrom(backfill.Progress{JobID: "a", Token: 7, Interval: "day", Done: 1, Failed: 2, Total: 3, ETA: 1500 * time.Millisecond, State: "running"})
	if p.JobID != "a" || p.Token != 7 || p.Interval != "day" || p.Done != 1 || p.Failed != 2 || p.Total != 3 || p.ETAMs != 1500 || p.State != "running" {
		t.Fatalf("progress %+v", p)
	}
	b, _ := json.Marshal(p)
	if !json.Valid(b) || !strings.Contains(string(b), `"etaMs":1500`) {
		t.Fatalf("json %s", b)
	}
}

// Ruling 67: a dropped order.update is recovered by refreshing the order book (orders.changed is the truth).
func TestForwardOrderUpdate_DropsTriggerRefresh(t *testing.T) {
	e := newSvcEnv(t)
	release := make(chan struct{})
	var once sync.Once
	startSource(t, e, EvOrderUpdate, func(_ string, _ any) { once.Do(func() { <-release }) })
	for i := 0; i < 400; i++ {
		bus.Publish(e.bus, kite.TopicOrderUpdate, kite.Order{OrderID: fmt.Sprint(i)})
	}
	close(release)
	eventually(t, "orders refresh after drops", func() bool {
		e.sync.mu.Lock()
		defer e.sync.mu.Unlock()
		return len(e.sync.refreshed) > 0 && e.sync.refreshed[0] == orders.ScopeOrders
	})
}

// Ruling 67: when live and polled quotes of one token share a window, the newer quote (higher TsMs) wins,
// whatever order the two subscriptions deliver them in.
func TestForwardTicks_KeepsHigherTsMs(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	bus.Publish(e.bus, ticker.TopicTicks, []ticker.Tick{{Token: 5, LTP: 50, ExchangeTS: time.UnixMilli(1000)}, {Token: 6, LTP: 60, ExchangeTS: time.UnixMilli(3000)}})
	bus.Publish(e.bus, TopicPolledTicks, []TickDTO{{Token: 5, LTP: 51, TsMs: 2000}, {Token: 6, LTP: 61, TsMs: 2000}})
	time.Sleep(50 * time.Millisecond)
	got := advanceUntilEvent(t, e, ch, EvTickBatch).payload.([]TickDTO)
	if len(got) != 2 || got[0].Token != 5 || got[0].LTP != 51 || !got[0].Polled || got[1].Token != 6 || got[1].LTP != 60 || got[1].Polled {
		t.Fatalf("%+v", got)
	}
}

// Ruling 67: at 08:59:30 the clock wakes at the 09:00 pre-open boundary itself. The 1-minute safety poll
// would wake at 09:00:30, so an event stamped exactly 09:00:00 after a 30 s advance can only come from
// the boundary timer.
func TestMarketStatus_PreOpenAtBoundaryNotPoll(t *testing.T) {
	e := newSvcEnv(t)
	e.svc.d.Calendar = mustCalendar(t)
	e.clock.Set(time.Date(2026, 10, 12, 8, 59, 30, 0, clock.IST))
	ch := startEvents(t, e)
	if first := nextEvent(t, ch, EvMarketStatus).payload.(MarketStatus); first.Phase != "closed" || first.DayType != "trading" {
		t.Fatalf("%+v", first)
	}
	e.clock.Advance(30 * time.Second)
	second := nextEvent(t, ch, EvMarketStatus).payload.(MarketStatus)
	if second.Phase != "pre-open" || second.Label != "Pre-open" || second.AtMs != ist(2026, 10, 12, 9, 0).UnixMilli() {
		t.Fatalf("%+v", second)
	}
}

// On a holiday or a weekend nothing opens at 09:00: no market.status event at the boundary or the poll.
func TestMarketStatus_HolidayAndWeekendStayClosed(t *testing.T) {
	for _, c := range []struct {
		name string
		day  int
		typ  string
	}{{"holiday", 20, "holiday"}, {"weekend", 17, "weekend"}} {
		t.Run(c.name, func(t *testing.T) {
			e := newSvcEnv(t)
			e.svc.d.Calendar = mustCalendar(t)
			e.clock.Set(time.Date(2026, 10, c.day, 8, 59, 30, 0, clock.IST))
			ch := startEvents(t, e)
			if first := nextEvent(t, ch, EvMarketStatus).payload.(MarketStatus); first.Phase != "closed" || first.DayType != c.typ {
				t.Fatalf("%+v", first)
			}
			for i := 0; i < 3; i++ {
				e.clock.Advance(30 * time.Second)
				select {
				case ev := <-ch:
					if ev.name == EvMarketStatus {
						t.Fatalf("unexpected %+v", ev)
					}
				case <-time.After(30 * time.Millisecond):
				}
			}
		})
	}
}

func TestMarketClock_ExitsOnCancel(t *testing.T) {
	e := newSvcEnv(t)
	e.svc.d.Calendar = mustCalendar(t)
	sub := bus.Subscribe(e.bus, TopicMarketStatus, bus.SubOpts[MarketStatus]{Name: "test"})
	cancel := startSource(t, e, EvMarketStatus, func(string, any) {})
	if _, ok := <-sub.C(); !ok {
		t.Fatal("no initial status")
	}
	if !e.clock.WaitTimers(1, time.Second) {
		t.Fatal("market clock armed no timer")
	}
	cancel()
	eventually(t, "market clock timer stopped", func() bool { return e.clock.PendingTimers() == 0 })
	e.clock.Advance(6 * time.Hour)
	select {
	case v := <-sub.C():
		t.Fatalf("published after cancel: %+v", v)
	case <-time.After(50 * time.Millisecond):
	}
}
