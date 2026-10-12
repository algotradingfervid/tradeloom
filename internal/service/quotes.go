// internal/service/quotes.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/feed"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/logging"
)

const (
	quotesStage       = "quotes"
	UnsubscribeLinger = 30 * time.Second
	QuotePollInterval = 5 * time.Second
	QuoteCacheFlush   = 60 * time.Second
	QuoteCacheKey     = "quotes.last" // window_state key: JSON map token → TickDTO
	ltpChunk          = 500
	// quotePollPriority queues background quote polls behind interactive requests (as orders.Sync does, Ruling 62).
	quotePollPriority = ratelimit.Recorder
)

var liveConsumers = map[string]ticker.Consumer{
	"chart":     ticker.ConsumerChart,
	"watchlist": ticker.ConsumerWatchlist,
	"widget":    ticker.ConsumerWidget,
}

// WatchResult tells the UI which tokens are live, which are polled, and the last known quote of each.
type WatchResult struct {
	Live     []uint32  `json:"live"`
	Polled   []uint32  `json:"polled"`
	Snapshot []TickDTO `json:"snapshot"`
}

// WatchQuotes replaces a UI consumer's quote set (FR-WATCH-006): new tokens now, removed tokens after
// UnsubscribeLinger; tokens the feed cannot carry are polled by RunQuotes.
func (s *Service) WatchQuotes(_ context.Context, consumer string, tokens []uint32) (WatchResult, error) {
	c, ok := liveConsumers[consumer]
	if !ok {
		return WatchResult{}, refused(quotesStage, "invalid_consumer", "Unknown quote consumer "+consumer+".")
	}
	s.quotesOnce.Do(s.loadQuoteCache)
	want := make(map[uint32]bool, len(tokens))
	ordered := make([]uint32, 0, len(tokens))
	for _, t := range tokens {
		if !want[t] {
			want[t] = true
			ordered = append(ordered, t)
		}
	}

	// watchMu keeps the want-set, the poll sets and the feed's subscriptions consistent: a concurrent call or
	// a linger expiry can never interleave between computing a change and applying it (Ruling 62 pattern).
	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	s.liveMu.Lock()
	cur := s.live[c]
	var add, removed []uint32
	for _, t := range ordered {
		if !cur[t] {
			add = append(add, t)
		}
	}
	for t := range cur {
		if !want[t] {
			removed = append(removed, t)
		}
	}
	s.liveMu.Unlock()
	sort.Slice(add, func(i, j int) bool { return add[i] < add[j] })

	var toPoll []uint32
	deferred := false
	if len(add) > 0 {
		if s.d.Session.State().DataPlan == kite.PlanNone {
			toPoll = add // C-01 / FR-ACCT-007: no WebSocket on a Personal plan
		} else {
			err := s.d.Live.Subscribe(c, consumer, add, ticker.ModeQuote)
			var budget *ticker.ErrBudget
			switch {
			case err == nil:
			case errors.Is(err, feed.ErrNotRunning):
				toPoll, deferred = add, true
			case errors.As(err, &budget):
				toPoll = add
				s.d.Log.Warn("quote subscription over budget; polling instead", "consumer", consumer, "tokens", len(add), "err", err)
			default:
				// The consumer's set is left as it was (no half-applied change): a retry computes the same additions.
				return WatchResult{}, s.refuse(quotesStage, refused(quotesStage, "subscription_failed", err.Error()))
			}
		}
	}
	s.liveMu.Lock()
	s.live[c] = want
	s.liveMu.Unlock()

	var deadline time.Time
	if len(removed) > 0 {
		deadline = s.d.Clock.Now().Add(UnsubscribeLinger)
	}
	s.pollMu.Lock()
	s.initPollLocked()
	set := s.polled[c]
	if set == nil {
		set = map[uint32]bool{}
		s.polled[c] = set
	}
	def := s.deferred[c]
	if def == nil {
		def = map[uint32]bool{}
		s.deferred[c] = def
	}
	pend := s.pending[c]
	if pend == nil {
		pend = map[uint32]time.Time{}
		s.pending[c] = pend
	}
	for _, t := range add {
		delete(set, t)
		delete(def, t)
		delete(pend, t) // re-added within the linger window: keep it
	}
	for _, t := range toPoll {
		set[t] = true
		if deferred {
			def[t] = true
		}
	}
	for _, t := range removed {
		pend[t] = deadline
	}
	res := WatchResult{Live: []uint32{}, Polled: []uint32{}}
	for _, t := range ordered {
		if set[t] {
			res.Polled = append(res.Polled, t)
		} else {
			res.Live = append(res.Live, t)
		}
	}
	s.pollMu.Unlock()
	res.Snapshot = s.LastQuotes(ordered)

	if len(removed) > 0 {
		quit := s.quotesQuit()
		linger := s.d.Clock.At(deadline)
		logging.Go("service.quotes.linger", func() {
			select {
			case <-linger.C():
				s.expireLinger(c, consumer, removed, deadline)
			case <-quit: // RunQuotes ended (shutdown): keep the subscriptions, release the timer
				linger.Stop()
			}
		})
	}
	return res, nil
}

// expireLinger unsubscribes the removed tokens whose linger deadline is still the one armed (not re-added,
// not removed again later).
func (s *Service) expireLinger(c ticker.Consumer, consumer string, removed []uint32, deadline time.Time) {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()
	s.liveMu.Lock()
	wanted := s.live[c]
	s.liveMu.Unlock()
	var drop []uint32
	s.pollMu.Lock()
	for _, t := range removed {
		at, ok := s.pending[c][t]
		if !ok || !at.Equal(deadline) || wanted[t] {
			continue
		}
		delete(s.pending[c], t)
		delete(s.polled[c], t)
		delete(s.deferred[c], t)
		drop = append(drop, t)
	}
	s.pollMu.Unlock()
	if len(drop) > 0 {
		s.d.Live.Unsubscribe(c, consumer, drop)
	}
}

// initPollLocked creates the poll-state maps on first use (pollMu held).
func (s *Service) initPollLocked() {
	if s.polled == nil {
		s.polled = map[ticker.Consumer]map[uint32]bool{}
		s.deferred = map[ticker.Consumer]map[uint32]bool{}
		s.pending = map[ticker.Consumer]map[uint32]time.Time{}
		s.closes = map[uint32]float64{}
		s.quitQuotes = make(chan struct{})
	}
}

// quotesQuit is closed when RunQuotes exits; pending linger goroutines end with it.
func (s *Service) quotesQuit() <-chan struct{} {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	s.initPollLocked()
	return s.quitQuotes
}

// RunQuotes polls the REST quote endpoints for polled tokens every QuotePollInterval and flushes the
// last-quote cache every QuoteCacheFlush and on exit.
func (s *Service) RunQuotes(ctx context.Context) {
	s.quotesOnce.Do(s.loadQuoteCache)
	clk := s.d.Clock
	poll := clk.At(clk.Now().Add(QuotePollInterval))
	flush := clk.At(clk.Now().Add(QuoteCacheFlush))
	defer func() {
		poll.Stop()
		flush.Stop()
		s.quitOnce.Do(func() {
			s.pollMu.Lock()
			s.initPollLocked()
			close(s.quitQuotes)
			s.pollMu.Unlock()
		})
		s.flushQuoteCache(context.Background())
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C():
			poll = clk.At(clk.Now().Add(QuotePollInterval))
			s.pollQuotes(ctx)
		case <-flush.C():
			flush = clk.At(clk.Now().Add(QuoteCacheFlush))
			s.flushQuoteCache(ctx)
		}
	}
}

// polledTokens is the union of every consumer's polled set; deferred tokens leave it once the feed is live.
func (s *Service) polledTokens() []uint32 {
	feedLive := s.FeedState().State == "live"
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	s.initPollLocked()
	union := map[uint32]bool{}
	for c, set := range s.polled {
		for t := range set {
			if feedLive && s.deferred[c][t] {
				delete(set, t)
				delete(s.deferred[c], t)
				continue
			}
			union[t] = true
		}
	}
	out := make([]uint32, 0, len(union))
	for t := range union {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (s *Service) pollQuotes(ctx context.Context) {
	if s.d.Quotes == nil {
		return
	}
	st := s.d.Session.State()
	if st.State != kite.StateReady {
		return
	}
	s.pollMu.Lock()
	s.initPollLocked()
	off := s.pollOff != "" && s.pollOff == st.SessionID
	if s.closeSession != st.SessionID { // previous closes belong to the login (trading day) they were fetched in
		s.closeSession, s.closes = st.SessionID, map[uint32]float64{}
	}
	s.pollMu.Unlock()
	if off {
		return // Kite refused quotes for this login: the UI keeps the last known values (FR-ACCT-007)
	}
	tokens := s.polledTokens()
	if len(tokens) == 0 {
		return
	}
	ids := make([]string, 0, len(tokens))
	byID := map[string]uint32{}
	var unknown []uint32
	for _, t := range tokens {
		in, ok := s.d.Instruments.ByToken(t)
		if !ok {
			unknown = append(unknown, t)
			continue
		}
		id := in.Exchange + ":" + in.Tradingsymbol
		ids = append(ids, id)
		byID[id] = t
	}
	if len(unknown) > 0 {
		s.d.Log.Debug("quote poll skips tokens missing from the instrument master", "count", len(unknown), "tokens", unknown[:min(len(unknown), 20)])
	}
	if len(ids) == 0 {
		return
	}
	bg := ratelimit.WithPriority(ctx, quotePollPriority)

	// The previous close is fetched once per token per session; a cached (possibly restored) close is only
	// the fallback while OHLC fails.
	s.pollMu.Lock()
	var needClose []string
	for _, id := range ids {
		if s.closes[byID[id]] <= 0 {
			needClose = append(needClose, id)
		}
	}
	s.pollMu.Unlock()
	for i := 0; i < len(needClose); i += ltpChunk {
		end := min(i+ltpChunk, len(needClose))
		q, err := s.d.Quotes.OHLC(bg, needClose[i:end]...)
		if err != nil {
			if s.quotesRefused(st.SessionID, err) {
				return
			}
			s.pollFailed(ctx, "quote OHLC poll failed", err)
			break
		}
		s.pollMu.Lock()
		if s.closeSession == st.SessionID {
			for _, v := range q {
				if v.OHLC.Close > 0 {
					s.closes[uint32(v.InstrumentToken)] = v.OHLC.Close
				}
			}
		}
		s.pollMu.Unlock()
	}
	s.pollMu.Lock()
	closes := make(map[uint32]float64, len(ids))
	for _, t := range byID {
		if c := s.closes[t]; c > 0 {
			closes[t] = c
		}
	}
	s.pollMu.Unlock()
	cached := map[uint32]TickDTO{}
	for _, q := range s.LastQuotes(tokens) {
		cached[q.Token] = q
	}

	now := s.d.Clock.Now().UnixMilli()
	var out []TickDTO
	defer func() {
		// Chunks that succeeded are published even when a later chunk failed.
		if len(out) > 0 {
			sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
			bus.Publish(s.d.Bus, TopicPolledTicks, out)
		}
	}()
	for i := 0; i < len(ids); i += ltpChunk {
		end := min(i+ltpChunk, len(ids))
		q, err := s.d.Quotes.LTP(bg, ids[i:end]...)
		if err != nil {
			if !s.quotesRefused(st.SessionID, err) {
				s.pollFailed(ctx, "quote LTP poll failed", err)
			}
			return
		}
		for _, v := range q {
			if v.LastPrice <= 0 {
				continue // no trade yet / bad quote: never overwrite a known price with 0
			}
			tok := uint32(v.InstrumentToken)
			d := TickDTO{Token: tok, LTP: v.LastPrice, Close: closes[tok], TsMs: now, Polled: true}
			if d.Close <= 0 {
				d.Close = cached[tok].Close
			}
			if d.Close > 0 {
				d.Change = d.LTP - d.Close
				d.ChangePct = d.Change / d.Close * 100
			}
			out = append(out, d)
		}
	}
}

// pollFailed logs a poll failure at Warn unless the poll was cancelled (logout, shutdown).
func (s *Service) pollFailed(ctx context.Context, msg string, err error) {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		s.d.Log.Debug(msg, "err", err)
		return
	}
	s.d.Log.Warn(msg, "err", err)
}

// quotesRefused stops polling for this login when Kite says the key has no market-data permission.
func (s *Service) quotesRefused(sessionID string, err error) bool {
	if !kite.IsNoDataPermission(err) {
		return false
	}
	s.pollMu.Lock()
	s.pollOff = sessionID
	s.pollMu.Unlock()
	s.d.Log.Info("quote polling is not permitted on this API plan; showing the last known quotes")
	return true
}

// quotesLocked is the lazily initialised last-quote cache (Task 21 creates it on the first tick); quoteMu held.
func (s *Service) quotesLocked() map[uint32]TickDTO {
	if s.lastQuote == nil {
		s.lastQuote = map[uint32]TickDTO{}
	}
	return s.lastQuote
}

// loadQuoteCache seeds the in-memory cache from window_state (entries marked as not polled).
func (s *Service) loadQuoteCache() {
	if s.d.UiState == nil {
		return
	}
	raw, err := s.d.UiState.Get(context.Background(), QuoteCacheKey)
	if err != nil || raw == "" {
		if err != nil {
			s.d.Log.Warn("reading the saved quote cache failed", "err", err)
		}
		return
	}
	var m map[uint32]TickDTO
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		s.d.Log.Warn("ignoring the saved quote cache", "err", err)
		return
	}
	s.quoteMu.Lock()
	cache := s.quotesLocked()
	for tok, q := range m {
		if _, ok := cache[tok]; !ok && q.LTP > 0 {
			q.Token, q.Polled = tok, false
			cache[tok] = q
		}
	}
	s.quoteMu.Unlock()
}

// flushQuoteCache writes the cache to window_state when it changed since the last flush.
func (s *Service) flushQuoteCache(ctx context.Context) {
	if s.d.UiState == nil {
		return
	}
	s.quoteMu.Lock()
	if !s.quoteDirty {
		s.quoteMu.Unlock()
		return
	}
	cache := s.quotesLocked()
	snapshot := make(map[uint32]TickDTO, len(cache))
	for k, v := range cache {
		snapshot[k] = v
	}
	s.quoteDirty = false
	s.quoteMu.Unlock()
	b, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	if err := s.d.UiState.Set(ctx, QuoteCacheKey, string(b)); err != nil {
		s.d.Log.Warn("saving the quote cache failed", "err", err)
		s.quoteMu.Lock()
		s.quoteDirty = true
		s.quoteMu.Unlock()
	}
}
