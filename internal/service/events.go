// internal/service/events.go
package service

import (
	"context"
	"fmt"
	"reflect"
	"slices"
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
		// kite.Session publishes outside its lock, so payloads can arrive out of order (Rulings 59, 61):
		// the event is a wake-up and the bridge forwards the authoritative Session.State().
		forwardNow(s, kite.TopicSessionState, func(kite.SessionState) any { return s.statusFrom(s.d.Session.State()) }),
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
// A closed subscription channel (bus closed) ends the source.
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
				case v, ok := <-sub.C():
					if !ok {
						return
					}
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
// within CoalesceWindow (one entry per token, ordered by token) and records the last quote per token for Quotes.Watch snapshots.
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
				case batch, ok := <-live.C():
					if !ok {
						return
					}
					for _, t := range batch {
						add(TickDTOFrom(t))
					}
				case batch, ok := <-polled.C():
					if !ok {
						return
					}
					for _, d := range batch {
						d.Polled = true
						add(d)
					}
				case <-timerC(fire):
					fire = nil
					if len(acc.order) == 0 {
						continue
					}
					// Live and polled quotes arrive on two subscriptions whose relative order is not defined,
					// so the batch is ordered by token to keep the payload deterministic.
					slices.Sort(acc.order)
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
	if s.lastQuote == nil {
		s.lastQuote = map[uint32]TickDTO{}
	}
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
	if s.lastFeed.State == "" {
		return ticker.FeedState{State: "stopped"}
	}
	return s.lastFeed
}

// MarketStatus evaluates the NSE phase at app-clock now (FR-SHELL-005).
func (s *Service) MarketStatus() MarketStatus { return s.marketStatusAt(s.d.Clock.Now()) }

func (s *Service) marketStatusAt(now time.Time) MarketStatus {
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
			// publish evaluates the status at one instant and returns it, so the next wake-up is armed from
			// the same instant: a clock that moved meanwhile makes the timer fire at once instead of skipping
			// a boundary.
			publish := func() time.Time {
				now := s.d.Clock.Now()
				cur := s.marketStatusAt(now)
				if last.AtMs != 0 && cur.Phase == last.Phase && cur.DayType == last.DayType && cur.Label == last.Label && cur.Tooltip == last.Tooltip {
					return now
				}
				last = cur
				bus.Publish(s.d.Bus, TopicMarketStatus, cur)
				return now
			}
			at := publish()
			for {
				wake := s.d.Clock.At(s.nextMarketBoundary(at))
				select {
				case <-ctx.Done():
					wake.Stop()
					return
				case <-wake.C():
				}
				at = publish()
			}
		})
	}}
}

// Navigate asks the UI to show an in-app route (app.navigate). Anything that is not a route is ignored.
func (s *Service) Navigate(route string) {
	route = strings.TrimSpace(route)
	if !strings.HasPrefix(route, "/") || strings.ContainsAny(route, " \t\r\n") {
		// The rejected value comes from argv (it may be a pasted URL): log only its length.
		s.d.Log.Warn("navigation request ignored", "len", len(route))
		return
	}
	bus.Publish(s.d.Bus, TopicNavigate, Navigate{Route: route})
}
