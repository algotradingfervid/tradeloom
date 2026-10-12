> **P0-3 Session & Service Layer Implementation Plan — Task 21.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 21: Event catalogue, coalescing sources, market-status clock and the TypeScript event generator

**Files:**
- Create: `internal/service/events.go`, `internal/app/events.go`, `internal/app/eventsgen/main.go`
- Modify: `internal/service/deps.go` (Task 14: last-quote cache and last feed state on `Service`)
- Test: `internal/service/events_test.go`, `internal/app/eventsgen/main_test.go`
- Generated: `frontend/src/api/events.gen.ts`

**Interfaces:**
- Consumes: the topics `kite.TopicSessionState` (Task 1), `orders.TopicBook/TopicPositions/TopicHoldings` (Task 19), `service.TopicWatchlists` (Task 15) and the P0-2 / P0-1 topics in "Consumes" (`ticker.TopicFeed` (`ticker.FeedState`), `ticker.TopicTicks` (`[]ticker.Tick`), `builder.TopicForming`/`builder.TopicClosed` (`builder.Event`), `clock.TopicOffset` (`clock.Offset`), `backfill.TopicProgress` (`backfill.Progress`), `ratelimit.TopicUsage` (`ratelimit.Usage`), `settings.Topic` (`settings.Changed`), `kite.TopicOrderUpdate` (`kite.Order`)); P0-1 `bus.SubOpts[T]`, `bus.Unsubscribe`; P0-2 `clock.Timer`, `calendar.Service.PhaseAt/LabelAt/Day`, `calendar.DateOf`, `(Date).AddDays`, `calendar.NSE`, `calendar.PhaseClosed`; Task 16 `TickDTO`, `TickDTOFrom`, `CandleEventFrom`, `mustCalendar` (test helper).
- Produces:
  ```go
  type Emit func(name string, payload any)
  type EventSpec struct{ Name string; Payload reflect.Type; Immediate bool }
  type EventSource struct{ Name string; Start func(ctx context.Context, emit Emit) } // Start subscribes synchronously, then runs via logging.Go
  const EvSessionState = "session.state" /* … one constant per row of the Events table in "Produces", incl. */ EvOrderUpdate = "order.update"; EvMarketStatus = "market.status"; EvJobProgress = "job.progress"; EvAppNavigate = "app.navigate"
  const CoalesceWindow = 100 * time.Millisecond
  type MarketStatus struct{ Exchange, Phase, DayType, Label, Tooltip string; AtMs int64 } // TS service.MarketStatus
  type JobProgress struct{ JobID, Kind, Stage string; Done, Total int; Finished bool; Error, ResultPath string } // TS service.JobProgress
  type Navigate struct{ Route string }                                                                        // TS service.Navigate
  var TopicMarketStatus bus.Topic[MarketStatus]; var TopicJobProgress bus.Topic[JobProgress]; var TopicNavigate bus.Topic[Navigate]
  var TopicPolledTicks bus.Topic[[]TickDTO] // "tick.polled": REST-polled quotes (Task 29) merged into tick.batch; not a UI event
  func EventCatalogue() []EventSpec
  func (s *Service) EventSources() []EventSource
  func (s *Service) MarketStatus() MarketStatus      // NSE phase at app-clock now
  func (s *Service) FeedState() ticker.FeedState     // last feed.state seen (stopped before the first)
  func (s *Service) LastQuotes(tokens []uint32) []TickDTO // last quote per token from the in-memory cache
  func (s *Service) Navigate(route string)           // publishes app.navigate for routes starting with "/"
  // eventsgen: func Generate(specs []service.EventSpec) string  → frontend/src/api/events.gen.ts
  ```

**Deviation, flagged for the consistency reviewer:** ARCH §3.4/§8.4 place the coalescing bridge in `internal/app/bridge.go`. That would make `internal/app` import `ticker`, `clock`, `candles/builder`, `backfill`, `ratelimit` and `orders` directly, which breaks rule 7 ("service is the only entry for app"). The coalescing therefore lives in `internal/service/events.go` (transport-neutral, so P5 Telegram can reuse it). `internal/app` only supplies the `Emit` function (Task 22). Event names and payloads are unchanged. Also, ARCH §3.2's `session.ready` / `session.loggedOut` are not separate events: both are `session.state` with `state` = `ready` / `logged_out` (the single topic of §3.4).

Four events are added here for P0-4: `order.update` forwards P0-2's `kite.TopicOrderUpdate` unchanged (the raw gokiteconnect `Order`, snake_case JSON); `market.status` is produced by this task's market clock (NSE on the app clock, evaluated at start, at every phase boundary and every minute as a safety net for clock jumps, published on change); `job.progress` is published by the jobs of Tasks 30 and 32; `app.navigate` by `App.SecondInstance` (Task 23) through `Navigate`. The tick path merges the WebSocket batches with the polled quotes of Task 29 and keeps the last quote per token for `Quotes.Watch` snapshots.

- [ ] **Step 1: Write the failing tests**

```go
// internal/service/events_test.go
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

func TestForwardNow_SessionStateEveryTransition(t *testing.T) {
	e := newSvcEnv(t)
	ch := startEvents(t, e)
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateLoggingIn})
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect})
	first := nextEvent(t, ch, EvSessionState).payload.(SessionStatus)
	second := nextEvent(t, ch, EvSessionState).payload.(SessionStatus)
	if first.State != "logging_in" || second.State != "ready" || second.Credentials.RedirectURL == "" {
		t.Fatalf("%+v / %+v", first, second)
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
```

```go
// internal/app/eventsgen/main_test.go
package main

import (
	"strings"
	"testing"

	"tradeloom/internal/service"
)

func TestGenerate(t *testing.T) {
	out := Generate(service.EventCatalogue())
	for _, want := range []string{
		"// Code generated by internal/app/eventsgen. DO NOT EDIT.",
		"export interface service_SessionStatus {",
		"  credentials: kite_CredentialsInfo;",
		"  expiresAtMs: number;",
		"export interface kite_Order {",
		"  order_id: string;",
		"  order_timestamp: string;",
		"  meta: Record<string, unknown>;",
		"export interface service_MarketStatus {",
		"  atMs: number;",
		`  "session.state": service_SessionStatus;`,
		`  "tick.batch": Array<service_TickDTO>;`,
		`  "orders.changed": orders_BookSnapshot;`,
		`  "order.update": kite_Order;`,
		`  "market.status": service_MarketStatus;`,
		`  "job.progress": service_JobProgress;`,
		`  "app.navigate": service_Navigate;`,
		`  sessionState: "session.state",`,
		`  appNavigate: "app.navigate",`,
		"export type EventName = keyof EventPayloads;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "v4_Order") {
		t.Error("gokiteconnect types must be namespaced kite_, not v4_")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/service/ ./internal/app/eventsgen/ -run 'TestEventSources|TestForward|TestMarketStatus|TestNavigate|TestGenerate' -v`
Expected: FAIL to compile with `undefined: EventCatalogue` and `no Go files in …/internal/app/eventsgen`.

- [ ] **Step 3: Extend `internal/service/deps.go`**

Add to `Service` (after `liveCharts`):

```go
	quoteMu    sync.Mutex
	lastQuote  map[uint32]TickDTO // last quote per token (tick.batch, live or polled); Task 29 persists it
	quoteDirty bool

	feedMu   sync.Mutex
	lastFeed ticker.FeedState // last feed.state (System.Status)
```

and initialise them in `New`: `lastQuote: map[uint32]TickDTO{}`, `lastFeed: ticker.FeedState{State: "stopped"}`.

- [ ] **Step 4: Write the implementation**

```go
// internal/service/events.go
package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles/backfill"
	"tradeloom/internal/candles/builder"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/logging"
	"tradeloom/internal/orders"
	"tradeloom/internal/settings"
)

// Emit delivers one UI event (internal/app wraps runtime.EventsEmit).
type Emit func(name string, payload any)

type EventSpec struct {
	Name      string
	Payload   reflect.Type
	Immediate bool
}

type EventSource struct {
	Name  string
	Start func(ctx context.Context, emit Emit)
}

// Event names == bus topic names (ARCH §3.4).
const (
	EvSessionState      = "session.state"
	EvFeedState         = "feed.state"
	EvTickBatch         = "tick.batch"
	EvCandleForming     = "candle.forming"
	EvCandleClosed      = "candle.closed"
	EvClockOffset       = "clock.offset"
	EvBackfillProgress  = "backfill.progress"
	EvLimitsUsage       = "limits.usage"
	EvOrdersChanged     = "orders.changed"
	EvPositionsChanged  = "positions.changed"
	EvHoldingsChanged   = "holdings.changed"
	EvOrderUpdate       = "order.update"
	EvWatchlistsChanged = "watchlists.changed"
	EvSettingsChanged   = "settings.changed"
	EvMarketStatus      = "market.status"
	EvJobProgress       = "job.progress"
	EvAppNavigate       = "app.navigate"
)

const (
	CoalesceWindow = 100 * time.Millisecond
	marketPoll     = time.Minute // safety re-evaluation between phase boundaries (suspend/resume, offset corrections)
)

// MarketStatus is the status-indicator payload (event market.status; System.Status).
type MarketStatus struct {
	Exchange string `json:"exchange"`
	Phase    string `json:"phase"`
	DayType  string `json:"dayType"`
	Label    string `json:"label"`
	Tooltip  string `json:"tooltip"`
	AtMs     int64  `json:"atMs"`
}

// JobProgress reports a long-running job (backup, restore, support bundle).
type JobProgress struct {
	JobID      string `json:"jobId"`
	Kind       string `json:"kind"`
	Stage      string `json:"stage"`
	Done       int    `json:"done"`
	Total      int    `json:"total"`
	Finished   bool   `json:"finished"`
	Error      string `json:"error"`
	ResultPath string `json:"resultPath"`
}

// Navigate asks the UI router to show a route (a second launch with --view <route>).
type Navigate struct {
	Route string `json:"route"`
}

var (
	TopicMarketStatus = bus.NewTopic[MarketStatus]("market.status")
	TopicJobProgress  = bus.NewTopic[JobProgress]("job.progress")
	TopicNavigate     = bus.NewTopic[Navigate]("app.navigate")
	// TopicPolledTicks carries REST-polled quotes (Task 29) into the tick.batch path. It is not a UI event.
	TopicPolledTicks = bus.NewTopic[[]TickDTO]("tick.polled")
)

// EventCatalogue is the single declaration of UI events; eventsgen turns it into events.gen.ts.
func EventCatalogue() []EventSpec {
	return []EventSpec{
		{EvSessionState, reflect.TypeOf(SessionStatus{}), true},
		{EvFeedState, reflect.TypeOf(ticker.FeedState{}), false},
		{EvTickBatch, reflect.TypeOf([]TickDTO{}), false},
		{EvCandleForming, reflect.TypeOf([]CandleEventDTO{}), false},
		{EvCandleClosed, reflect.TypeOf(CandleEventDTO{}), true},
		{EvClockOffset, reflect.TypeOf(clock.Offset{}), false},
		{EvBackfillProgress, reflect.TypeOf([]backfill.Progress{}), false},
		{EvLimitsUsage, reflect.TypeOf(ratelimit.Usage{}), false},
		{EvOrdersChanged, reflect.TypeOf(orders.BookSnapshot{}), false},
		{EvPositionsChanged, reflect.TypeOf(orders.PositionsSnapshot{}), false},
		{EvHoldingsChanged, reflect.TypeOf(orders.HoldingsSnapshot{}), false},
		{EvOrderUpdate, reflect.TypeOf(kite.Order{}), true},
		{EvWatchlistsChanged, reflect.TypeOf(WatchlistsState{}), true},
		{EvSettingsChanged, reflect.TypeOf(settings.Changed{}), true},
		{EvMarketStatus, reflect.TypeOf(MarketStatus{}), true},
		{EvJobProgress, reflect.TypeOf(JobProgress{}), true},
		{EvAppNavigate, reflect.TypeOf(Navigate{}), true},
	}
}

// EventSources returns one source per catalogue entry, in catalogue order.
func (s *Service) EventSources() []EventSource {
	return []EventSource{
		forwardNow(s, kite.TopicSessionState, func(st kite.SessionState) any { return s.statusFrom(st) }),
		forwardLatest(s, ticker.TopicFeed, func(v ticker.FeedState) any { s.rememberFeed(v); return v }),
		forwardTicks(s),
		forwardKeyed(s, builder.TopicForming, func(e builder.Event) string { return fmt.Sprintf("%d|%s", e.Token, e.Interval) }, CandleEventFrom),
		forwardNow(s, builder.TopicClosed, func(e builder.Event) any { return CandleEventFrom(e) }),
		forwardLatest(s, clock.TopicOffset, func(v clock.Offset) any { return v }),
		forwardKeyed(s, backfill.TopicProgress, func(p backfill.Progress) string { return p.JobID }, func(p backfill.Progress) backfill.Progress { return p }),
		forwardLatest(s, ratelimit.TopicUsage, func(v ratelimit.Usage) any { return v }),
		forwardLatest(s, orders.TopicBook, func(v orders.BookSnapshot) any { return v }),
		forwardLatest(s, orders.TopicPositions, func(v orders.PositionsSnapshot) any { return v }),
		forwardLatest(s, orders.TopicHoldings, func(v orders.HoldingsSnapshot) any { return v }),
		forwardNow(s, kite.TopicOrderUpdate, func(o kite.Order) any { return o }),
		forwardNow(s, TopicWatchlists, func(v WatchlistsState) any { return v }),
		forwardNow(s, settings.Topic, func(v settings.Changed) any { return v }),
		marketStatusSource(s),
		forwardNow(s, TopicJobProgress, func(v JobProgress) any { return v }),
		forwardNow(s, TopicNavigate, func(v Navigate) any { return v }),
	}
}

// timerC returns a timer's channel, or a nil channel (never ready) while no window is open.
func timerC(t clock.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C()
}

// forward is the coalescing core: immediate topics emit every value; others accumulate for CoalesceWindow.
func forward[T any, A any](s *Service, t bus.Topic[T], immediate bool, empty func() A, add func(A, T) A, flush func(A) any) EventSource {
	name := t.Name()
	return EventSource{Name: name, Start: func(ctx context.Context, emit Emit) {
		sub := bus.Subscribe(s.d.Bus, t, bus.SubOpts[T]{Name: "events." + name, Buffer: 256})
		logging.Go("events."+name, func() {
			defer bus.Unsubscribe(s.d.Bus, sub)
			acc, has := empty(), false
			var fire clock.Timer
			defer func() {
				if fire != nil {
					fire.Stop()
				}
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case v := <-sub.C():
					if immediate {
						emit(name, flush(add(empty(), v)))
						continue
					}
					acc, has = add(acc, v), true
					if fire == nil {
						fire = s.d.Clock.At(s.d.Clock.Now().Add(CoalesceWindow))
					}
				case <-timerC(fire):
					fire = nil
					if has {
						emit(name, flush(acc))
						acc, has = empty(), false
					}
				}
			}
		})
	}}
}

func latest[T any](_ T, v T) T { return v }
func zero[T any]() T           { var z T; return z }

func forwardNow[T any](s *Service, t bus.Topic[T], conv func(T) any) EventSource {
	return forward(s, t, true, zero[T], latest[T], conv)
}

func forwardLatest[T any](s *Service, t bus.Topic[T], conv func(T) any) EventSource {
	return forward(s, t, false, zero[T], latest[T], conv)
}

type keyed[K comparable, T any] struct {
	order []K
	m     map[K]T
}

func forwardKeyed[T any, K comparable, D any](s *Service, t bus.Topic[T], key func(T) K, conv func(T) D) EventSource {
	return forward(s, t, false,
		func() keyed[K, T] { return keyed[K, T]{m: map[K]T{}} },
		func(a keyed[K, T], v T) keyed[K, T] {
			k := key(v)
			if _, ok := a.m[k]; !ok {
				a.order = append(a.order, k)
			}
			a.m[k] = v
			return a
		},
		func(a keyed[K, T]) any {
			out := make([]D, 0, len(a.order))
			for _, k := range a.order {
				out = append(out, conv(a.m[k]))
			}
			return out
		})
}

// forwardTicks merges WebSocket batches (ticker.TopicTicks) and REST-polled quotes (TopicPolledTicks) by token
// within CoalesceWindow and records the last quote per token for Quotes.Watch snapshots.
func forwardTicks(s *Service) EventSource {
	return EventSource{Name: EvTickBatch, Start: func(ctx context.Context, emit Emit) {
		live := bus.Subscribe(s.d.Bus, ticker.TopicTicks, bus.SubOpts[[]ticker.Tick]{Name: "events.tick.batch", Buffer: 256})
		polled := bus.Subscribe(s.d.Bus, TopicPolledTicks, bus.SubOpts[[]TickDTO]{Name: "events.tick.polled", Buffer: 64})
		logging.Go("events."+EvTickBatch, func() {
			defer bus.Unsubscribe(s.d.Bus, live)
			defer bus.Unsubscribe(s.d.Bus, polled)
			acc := keyed[uint32, TickDTO]{m: map[uint32]TickDTO{}}
			var fire clock.Timer
			defer func() {
				if fire != nil {
					fire.Stop()
				}
			}()
			add := func(d TickDTO) {
				if _, ok := acc.m[d.Token]; !ok {
					acc.order = append(acc.order, d.Token)
				}
				acc.m[d.Token] = d
				if fire == nil {
					fire = s.d.Clock.At(s.d.Clock.Now().Add(CoalesceWindow))
				}
			}
			for {
				select {
				case <-ctx.Done():
					return
				case batch := <-live.C():
					for _, t := range batch {
						add(TickDTOFrom(t))
					}
				case batch := <-polled.C():
					for _, d := range batch {
						d.Polled = true
						add(d)
					}
				case <-timerC(fire):
					fire = nil
					if len(acc.order) == 0 {
						continue
					}
					out := make([]TickDTO, 0, len(acc.order))
					for _, k := range acc.order {
						out = append(out, acc.m[k])
					}
					s.rememberTicks(out)
					emit(EvTickBatch, out)
					acc = keyed[uint32, TickDTO]{m: map[uint32]TickDTO{}}
				}
			}
		})
	}}
}

// rememberTicks keeps the last quote per token (snapshots for Quotes.Watch; persisted by Task 29).
func (s *Service) rememberTicks(ts []TickDTO) {
	s.quoteMu.Lock()
	for _, t := range ts {
		s.lastQuote[t.Token] = t
	}
	s.quoteDirty = true
	s.quoteMu.Unlock()
}

// LastQuotes returns the cached last quote of each token (in the order given); unknown tokens are omitted.
func (s *Service) LastQuotes(tokens []uint32) []TickDTO {
	s.quoteMu.Lock()
	defer s.quoteMu.Unlock()
	out := make([]TickDTO, 0, len(tokens))
	for _, t := range tokens {
		if q, ok := s.lastQuote[t]; ok {
			out = append(out, q)
		}
	}
	return out
}

func (s *Service) rememberFeed(v ticker.FeedState) {
	s.feedMu.Lock()
	s.lastFeed = v
	s.feedMu.Unlock()
}

// FeedState is the last feed.state payload seen ("stopped" before the first one).
func (s *Service) FeedState() ticker.FeedState {
	s.feedMu.Lock()
	defer s.feedMu.Unlock()
	return s.lastFeed
}

// MarketStatus evaluates the NSE phase at app-clock now (FR-SHELL-005).
func (s *Service) MarketStatus() MarketStatus {
	now := s.d.Clock.Now()
	st := MarketStatus{Exchange: string(calendar.NSE), Phase: string(calendar.PhaseClosed), DayType: string(calendar.DayTrading), Label: "Closed", AtMs: now.UnixMilli()}
	if s.d.Calendar == nil {
		return st
	}
	ph, day := s.d.Calendar.PhaseAt(calendar.NSE, now)
	lb := s.d.Calendar.LabelAt(calendar.NSE, now)
	st.Phase, st.DayType, st.Label, st.Tooltip = string(ph.Kind), string(day.Type), lb.Text, lb.Tooltip
	return st
}

// nextMarketBoundary is the next instant the phase can change: the current phase's end, else the start of the
// first phase of the next day that has one (up to 10 days ahead), capped by marketPoll.
func (s *Service) nextMarketBoundary(now time.Time) time.Time {
	next := now.Add(marketPoll)
	ph, _ := s.d.Calendar.PhaseAt(calendar.NSE, now)
	boundary := ph.End
	if !boundary.After(now) {
		d := calendar.DateOf(now)
		for i := 1; i <= 10 && !boundary.After(now); i++ {
			if day := s.d.Calendar.Day(calendar.NSE, d.AddDays(i)); len(day.Phases) > 0 {
				boundary = day.Phases[0].Start
			}
		}
	}
	if boundary.After(now) && boundary.Before(next) {
		next = boundary
	}
	return next
}

// marketStatusSource forwards TopicMarketStatus and runs the market clock that feeds it.
func marketStatusSource(s *Service) EventSource {
	fwd := forwardNow(s, TopicMarketStatus, func(v MarketStatus) any { return v })
	return EventSource{Name: EvMarketStatus, Start: func(ctx context.Context, emit Emit) {
		fwd.Start(ctx, emit)
		if s.d.Calendar == nil {
			return
		}
		logging.Go("service.market.clock", func() {
			var last MarketStatus
			publish := func() {
				cur := s.MarketStatus()
				if last.AtMs != 0 && cur.Phase == last.Phase && cur.DayType == last.DayType && cur.Label == last.Label && cur.Tooltip == last.Tooltip {
					return
				}
				last = cur
				bus.Publish(s.d.Bus, TopicMarketStatus, cur)
			}
			publish()
			for {
				wake := s.d.Clock.At(s.nextMarketBoundary(s.d.Clock.Now()))
				select {
				case <-ctx.Done():
					wake.Stop()
					return
				case <-wake.C():
				}
				publish()
			}
		})
	}}
}

// Navigate asks the UI to show an in-app route (app.navigate). Anything that is not a route is ignored.
func (s *Service) Navigate(route string) {
	route = strings.TrimSpace(route)
	if !strings.HasPrefix(route, "/") || strings.ContainsAny(route, " \t\r\n") {
		s.d.Log.Warn("navigation request ignored", "route", route)
		return
	}
	bus.Publish(s.d.Bus, TopicNavigate, Navigate{Route: route})
}
```

```go
// internal/app/events.go
package app

// Regenerate frontend/src/api/events.gen.ts from service.EventCatalogue():
//go:generate go run -tags webkit2_41,noorders ./eventsgen -out ../../frontend/src/api/events.gen.ts
```

```go
// internal/app/eventsgen/main.go
package main

import (
	"encoding"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"tradeloom/internal/service"
)

func main() {
	out := flag.String("out", "frontend/src/api/events.gen.ts", "output file")
	flag.Parse()
	if err := os.WriteFile(*out, []byte(Generate(service.EventCatalogue())), 0o644); err != nil {
		log.Fatal(err)
	}
}

var (
	timeType          = reflect.TypeOf(time.Time{})
	rawType           = reflect.TypeOf(json.RawMessage{})
	jsonMarshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

// pkgAlias names third-party payload packages the way the Go aliases do (kite.Order, not v4_Order).
var pkgAlias = map[string]string{
	"github.com/zerodha/gokiteconnect/v4":        "kite",
	"github.com/zerodha/gokiteconnect/v4/models": "kite",
}

type gen struct {
	defs  map[string]string
	order []string
}

func marshalsAsString(t reflect.Type) bool {
	return t.Implements(jsonMarshalerType) || reflect.PointerTo(t).Implements(jsonMarshalerType) ||
		t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType)
}

func (g *gen) ts(t reflect.Type) string {
	switch t {
	case timeType:
		return "string"
	case rawType:
		return "unknown"
	}
	if t.Kind() != reflect.Interface && marshalsAsString(t) {
		return "string" // models.Time (promoted time.Time.MarshalJSON), calendar.HM (MarshalText), …
	}
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "Array<" + g.ts(t.Elem()) + ">"
	case reflect.Map:
		return "Record<string, " + g.ts(t.Elem()) + ">"
	case reflect.Pointer:
		return g.ts(t.Elem()) + " | null"
	case reflect.Struct:
		return g.structType(t)
	}
	return "unknown"
}

func pkgName(t reflect.Type) string {
	if alias, ok := pkgAlias[t.PkgPath()]; ok {
		return alias
	}
	return path.Base(t.PkgPath())
}

func (g *gen) structType(t reflect.Type) string {
	name := pkgName(t) + "_" + t.Name()
	if _, ok := g.defs[name]; ok {
		return name
	}
	g.defs[name] = ""
	var b strings.Builder
	fmt.Fprintf(&b, "export interface %s {\n", name)
	g.fields(&b, t)
	b.WriteString("}\n")
	g.defs[name] = b.String()
	g.order = append(g.order, name)
	return name
}

func (g *gen) fields(b *strings.Builder, t reflect.Type) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" && f.Type.Kind() == reflect.Struct {
			g.fields(b, f.Type)
			continue
		}
		if name == "" {
			name = f.Name
		}
		opt := ""
		if strings.Contains(opts, "omitempty") {
			opt = "?"
		}
		fmt.Fprintf(b, "  %s%s: %s;\n", name, opt, g.ts(f.Type))
	}
}

func camel(s string) string {
	parts := strings.Split(s, ".")
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

// Generate renders event names and payload interfaces as TypeScript.
func Generate(specs []service.EventSpec) string {
	g := &gen{defs: map[string]string{}}
	var names, payloads strings.Builder
	for _, s := range specs {
		ty := g.ts(s.Payload)
		fmt.Fprintf(&names, "  %s: %q,\n", camel(s.Name), s.Name)
		fmt.Fprintf(&payloads, "  %q: %s;\n", s.Name, ty)
	}
	var out strings.Builder
	out.WriteString("// Code generated by internal/app/eventsgen. DO NOT EDIT.\n\n")
	for _, n := range g.order {
		out.WriteString(g.defs[n])
		out.WriteString("\n")
	}
	out.WriteString("export const EventNames = {\n" + names.String() + "} as const;\n\n")
	out.WriteString("export interface EventPayloads {\n" + payloads.String() + "}\n\n")
	out.WriteString("export type EventName = keyof EventPayloads;\n")
	return out.String()
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ ./internal/app/eventsgen/ -run 'TestEventSources|TestForward|TestMarketStatus|TestNavigate|TestGenerate' -v`
Expected: PASS (`TestEventSources_MatchCatalogue`, `TestForwardNow_SessionStateEveryTransition`, `TestForwardLatest_CoalescesWithin100ms`, `TestForwardTicks_MergesLiveAndPolledAndCaches`, `TestForwardKeyed_BackfillProgressPerJob`, `TestForwardNow_OrderUpdateAndFeedState`, `TestMarketStatus_PublishesAtStartAndAtPhaseBoundary`, `TestNavigate_PublishesRoutesOnly`, `TestGenerate`; `-race` clean).

- [ ] **Step 6: Generate the TypeScript file and type-check it**

Run: `mkdir -p frontend/src/api && go generate -tags webkit2_41,noorders ./internal/app/ && grep -c '^  "' frontend/src/api/events.gen.ts && cd frontend && npx tsc --noEmit --strict --skipLibCheck src/api/events.gen.ts`
Expected: `frontend/src/api/events.gen.ts` exists, starts with `// Code generated by internal/app/eventsgen. DO NOT EDIT.`, the `grep` prints `17` (one `EventPayloads` row per event), and `tsc` exits 0 with no output.

- [ ] **Step 7: Commit**

```bash
git add internal/service/events.go internal/service/events_test.go internal/service/deps.go internal/app/events.go internal/app/eventsgen frontend/src/api/events.gen.ts
git commit -m "feat(service,app): event catalogue with order, market, job and navigation events, merged tick path and TS event types

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
