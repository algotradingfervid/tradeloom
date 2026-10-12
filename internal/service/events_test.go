package service

import (
	"context"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/candles/backfill"
	"tradeloom/internal/kite"
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
	if got := advanceUntilEvent(t, e, ch, EvBackfillProgress).payload.([]backfill.Progress); len(got) != 2 {
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
	if fs := advanceUntilEvent(t, e, ch, EvFeedState).payload.(ticker.FeedState); fs.State != "live" {
		t.Fatalf("%+v", fs)
	}
	if e.svc.FeedState().Tokens != 3 {
		t.Fatalf("feed state not remembered: %+v", e.svc.FeedState())
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
