// internal/service/chart.go
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/candles/backfill"
	"tradeloom/internal/candles/builder"
	"tradeloom/internal/clock"
	"tradeloom/internal/instruments"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/feed"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/logging"
)

const (
	chartStage       = "chart"
	instrumentsStage = "instruments"
	maxSearchLimit   = 50
	defaultSearch    = 20
)

// maxRefreshSpan mirrors Kite's per-request historical limits (P0-2 backfill windows).
var maxRefreshSpan = map[candles.Interval]time.Duration{
	candles.Minute:   60 * 24 * time.Hour,
	candles.Minute3:  100 * 24 * time.Hour,
	candles.Minute5:  100 * 24 * time.Hour,
	candles.Minute10: 100 * 24 * time.Hour,
	candles.Minute15: 100 * 24 * time.Hour,
	candles.Minute30: 100 * 24 * time.Hour,
	candles.Minute60: 400 * 24 * time.Hour,
	candles.Day:      2000 * 24 * time.Hour,
}

type CandlesRequest struct {
	Token       uint32 `json:"token"`
	Interval    string `json:"interval"`
	FromMs      int64  `json:"fromMs"`
	ToMs        int64  `json:"toMs"`        // 0 = now
	TradingDays int    `json:"tradingDays"` // > 0: FromMs is resolved from the calendar
}

type RefreshRequest struct {
	Token    uint32 `json:"token"`
	Interval string `json:"interval"`
	FromMs   int64  `json:"fromMs"`
	ToMs     int64  `json:"toMs"`
}

type CandleDTO struct {
	TimeMs   int64   `json:"t"`
	Open     float64 `json:"o"`
	High     float64 `json:"h"`
	Low      float64 `json:"l"`
	Close    float64 `json:"c"`
	Volume   int64   `json:"v"`
	OI       int64   `json:"oi"`
	Complete bool    `json:"complete"`
}

type CandlesResult struct {
	Candles       []CandleDTO `json:"candles"`
	Complete      bool        `json:"complete"`
	BackfillJobID string      `json:"backfillJobId"`
	NeedsDataPlan bool        `json:"needsDataPlan"`
	Offline       bool        `json:"offline"`
}

// InstrumentDTO is instruments.Instrument with the expiry as Unix ms (§8.4).
type InstrumentDTO struct {
	Token          uint32 `json:"token"`
	Exchange       string `json:"exchange"`
	Segment        string `json:"segment"`
	Tag            string `json:"tag"`
	Tradingsymbol  string `json:"tradingsymbol"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	InstrumentType string `json:"instrumentType"`
	ExpiryMs       int64  `json:"expiryMs"`
	TickSizePaise  int64  `json:"tickSizePaise"`
	LotSize        int64  `json:"lotSize"`
	IsIndex        bool   `json:"isIndex"`
	IsFuture       bool   `json:"isFuture"`
	Underlying     string `json:"underlying"`
	AlsoOnNSE      bool   `json:"alsoOnNse"` // FR-WATCH-008: a BSE search hit whose company is also listed on NSE
}

// TickDTO is ticker.Tick reduced to what the UI shows; Close is the previous session close.
type TickDTO struct {
	Token     uint32  `json:"token"`
	LTP       float64 `json:"ltp"`
	Close     float64 `json:"close"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"changePct"`
	Volume    uint32  `json:"volume"`
	TsMs      int64   `json:"tsMs"`
	Polled    bool    `json:"polled"` // true when produced by REST polling instead of the WebSocket feed
}

type CandleEventDTO struct {
	Token    uint32    `json:"token"`
	Interval string    `json:"interval"`
	Candle   CandleDTO `json:"candle"`
	Partial  bool      `json:"partial"`
}

func CandleDTOFrom(c candles.Candle) CandleDTO {
	return CandleDTO{TimeMs: c.Start.UnixMilli(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume, OI: c.OI, Complete: c.Complete}
}

// TickDTOFrom stamps the tick with its exchange timestamp (full mode) or the app-clock arrival.
func TickDTOFrom(t ticker.Tick) TickDTO {
	d := TickDTO{Token: t.Token, LTP: t.LTP, Close: t.Close, Change: t.Change, Volume: t.Volume}
	switch {
	case !t.ExchangeTS.IsZero():
		d.TsMs = t.ExchangeTS.UnixMilli()
	case !t.Received.IsZero():
		d.TsMs = t.Received.UnixMilli()
	}
	if t.Close > 0 {
		d.ChangePct = (t.LTP - t.Close) / t.Close * 100
	}
	return d
}

func CandleEventFrom(e builder.Event) CandleEventDTO {
	return CandleEventDTO{Token: e.Token, Interval: string(e.Interval), Candle: CandleDTOFrom(e.Candle), Partial: e.Partial}
}

func instrumentDTO(in instruments.Instrument) InstrumentDTO {
	d := InstrumentDTO{Token: in.Token, Exchange: in.Exchange, Segment: in.Segment, Tag: string(in.Tag), Tradingsymbol: in.Tradingsymbol, Name: in.Name,
		DisplayName: in.DisplayName, InstrumentType: in.Type, TickSizePaise: in.TickSizePaise, LotSize: in.LotSize, IsIndex: in.IsIndex,
		IsFuture: in.IsFuture(), Underlying: in.Underlying}
	if !in.Expiry.IsZero() {
		d.ExpiryMs = in.Expiry.UnixMilli()
	}
	return d
}

// parseChartInterval accepts the eight stored (Kite) intervals. Resampled intervals (week, month,
// N-minute) are refused: the store's Read, the backfill and the builder handle native intervals only.
func parseChartInterval(s string) (candles.Interval, error) {
	iv, err := candles.ParseInterval(s)
	if err != nil || !iv.IsNative() {
		return "", refused(chartStage, "invalid_interval", "Unsupported timeframe "+s+".")
	}
	return iv, nil
}

// exchangeOf returns the calendar exchange of a token (NSE when unknown).
func (s *Service) exchangeOf(token uint32) calendar.Exchange {
	if in, ok := s.d.Instruments.ByToken(token); ok {
		for _, ex := range calendar.Exchanges {
			if string(ex) == in.Exchange {
				return ex
			}
		}
	}
	return calendar.NSE
}

// resolveRange turns a request into [from, to): ToMs 0 is now; TradingDays > 0 starts at the continuous open
// of the N-th trading day before to (the day of to counts when it is a trading day).
func (s *Service) resolveRange(token uint32, fromMs, toMs int64, tradingDays int) (time.Time, time.Time, error) {
	to := s.d.Clock.Now().In(clock.IST)
	if toMs > 0 {
		to = time.UnixMilli(toMs).In(clock.IST)
	}
	if tradingDays <= 0 {
		from := time.UnixMilli(fromMs).In(clock.IST)
		if !to.After(from) {
			return time.Time{}, time.Time{}, refused(chartStage, "invalid_range", "The start of the range must be before its end.")
		}
		return from, to, nil
	}
	if s.d.Calendar == nil {
		return time.Time{}, time.Time{}, refused(chartStage, "invalid_range", "Trading-day ranges need the market calendar.")
	}
	ex := s.exchangeOf(token)
	d := calendar.DateOf(to)
	if !s.d.Calendar.IsTradingDay(ex, d) {
		p, ok := s.d.Calendar.PrevTradingDay(ex, d)
		if !ok {
			return time.Time{}, time.Time{}, refused(chartStage, "invalid_range", "No trading day found before the end of the range.")
		}
		d = p
	}
	for i := 1; i < tradingDays; i++ {
		p, ok := s.d.Calendar.PrevTradingDay(ex, d)
		if !ok {
			break
		}
		d = p
	}
	from := d.Midnight()
	if ph, ok := s.d.Calendar.Day(ex, d).Phase(calendar.PhaseContinuous); ok {
		from = ph.Start
	}
	return from, to, nil
}

func (s *Service) readRange(ctx context.Context, token uint32, iv candles.Interval, from, to time.Time) (CandlesResult, error) {
	have, err := s.d.Candles.Read(ctx, token, iv, from, to)
	if err != nil {
		return CandlesResult{}, s.refuse(chartStage, err)
	}
	missing, err := s.d.Candles.Missing(ctx, token, iv, from, to)
	if err != nil {
		return CandlesResult{}, s.refuse(chartStage, err)
	}
	res := CandlesResult{Candles: make([]CandleDTO, 0, len(have)), Complete: len(missing) == 0}
	for _, c := range have {
		res.Candles = append(res.Candles, CandleDTOFrom(c))
	}
	return res, nil
}

// Candles serves the store and requests a backfill for gaps (FR-CHART-003 data path, FR-ACCT-007 degraded mode).
func (s *Service) Candles(ctx context.Context, req CandlesRequest) (CandlesResult, error) {
	iv, err := parseChartInterval(req.Interval)
	if err != nil {
		return CandlesResult{}, err
	}
	from, to, err := s.resolveRange(req.Token, req.FromMs, req.ToMs, req.TradingDays)
	if err != nil {
		return CandlesResult{}, err
	}
	res, err := s.readRange(ctx, req.Token, iv, from, to)
	if err != nil || res.Complete {
		return res, err
	}
	st := s.d.Session.State()
	switch {
	case st.State != kite.StateReady:
		res.Offline = true
	case st.DataPlan == kite.PlanNone:
		res.NeedsDataPlan = true
	default:
		in, known := s.d.Instruments.ByToken(req.Token)
		job := backfill.Job{Token: req.Token, Interval: iv, From: from, To: to, Requester: "chart", Priority: ratelimit.Interactive, OI: known && in.IsFuture()}
		id, err := s.d.Backfill.Submit(ctx, job)
		if err != nil {
			return res, s.refuse(chartStage, err)
		}
		res.BackfillJobID = id
	}
	return res, nil
}

// candlesFromHistorical converts Kite rows to store candles marked as a manual fetch.
// HasOI follows the request's OI flag, as P0-2 backfill stores it (a future's OI can genuinely be 0).
func candlesFromHistorical(hs []kite.HistoricalData, iv candles.Interval, oi bool, now time.Time) []candles.Candle {
	out := make([]candles.Candle, 0, len(hs))
	for _, h := range hs {
		start := h.Date.Time.In(clock.IST)
		out = append(out, candles.Candle{Start: start, Open: h.Open, High: h.High, Low: h.Low, Close: h.Close, Volume: int64(h.Volume),
			OI: int64(h.OI), HasOI: oi, Source: candles.SourceManual, Complete: !start.Add(iv.Duration()).After(now)})
	}
	return out
}

// RefreshCandles re-fetches one window from Kite at Interactive priority and upserts it as source manual (FR-CHART-006).
func (s *Service) RefreshCandles(ctx context.Context, req RefreshRequest) (CandlesResult, error) {
	iv, err := parseChartInterval(req.Interval)
	if err != nil {
		return CandlesResult{}, err
	}
	from, to, err := s.resolveRange(req.Token, req.FromMs, req.ToMs, 0)
	if err != nil {
		return CandlesResult{}, err
	}
	if span := maxRefreshSpan[iv]; span > 0 && to.Sub(from) > span {
		return CandlesResult{}, refused(chartStage, "invalid_range", "Refresh a smaller range: Kite serves at most "+span.String()+" of "+req.Interval+" candles per request.")
	}
	st := s.d.Session.State()
	switch {
	case st.State != kite.StateReady:
		return CandlesResult{}, refused(chartStage, "offline", "Log in to Kite to refresh candles.")
	case st.DataPlan == kite.PlanNone:
		return CandlesResult{}, refused(chartStage, "no_data_plan", "This API key has no market-data permission; cached candles only.")
	}
	in, known := s.d.Instruments.ByToken(req.Token)
	hreq := kite.HistoricalRequest{Token: int(req.Token), Interval: string(iv), From: from, To: to, OI: known && in.IsFuture()}
	_, rows, ok, err := s.d.Historical.HistoricalPick(ctx, ratelimit.Interactive, func() (kite.HistoricalRequest, bool) { return hreq, true })
	if err != nil {
		s.d.Log.Warn("manual candle refresh failed", "token", req.Token, "interval", iv, "err", err)
		return CandlesResult{}, refused(chartStage, "refresh_failed", logging.Default.RedactString("Kite did not return candles: "+err.Error()))
	}
	if ok {
		cs := candlesFromHistorical(rows, iv, hreq.OI, s.d.Clock.Now())
		if err := s.d.CandleWriter.Upsert(ctx, req.Token, iv, cs, candles.SourceManual); err != nil {
			return CandlesResult{}, s.refuse(chartStage, err)
		}
		s.d.Log.Info("candles refreshed manually", "token", req.Token, "interval", iv, "rows", len(cs))
	}
	return s.readRange(ctx, req.Token, iv, from, to)
}

// OpenLive subscribes a chart to live ticks and candle building for (token, interval); returns the subscription id.
func (s *Service) OpenLive(_ context.Context, token uint32, interval string) (string, error) {
	iv, err := parseChartInterval(interval)
	if err != nil {
		return "", err
	}
	if s.d.Session.State().DataPlan == kite.PlanNone {
		return "", refused(chartStage, "no_data_plan", "This API key has no market-data permission; live candles are unavailable.")
	}
	id := s.d.NewID()
	if err := s.d.Live.Subscribe(ticker.ConsumerChart, id, []uint32{token}, ticker.ModeQuote); err != nil && !errors.Is(err, feed.ErrNotRunning) {
		var budget *ticker.ErrBudget
		if errors.As(err, &budget) {
			return "", refused(chartStage, "subscription_budget", err.Error())
		}
		return "", s.refuse(chartStage, err)
	}
	// Track/Untrack run under chartMu (the builder never calls back into the service), so an
	// OpenLive racing the CloseLive of the same (token, interval) cannot leave the pair untracked.
	s.chartMu.Lock()
	defer s.chartMu.Unlock()
	first := !s.trackedLocked(token, iv)
	s.liveCharts[id] = liveChart{token: token, iv: iv}
	if first && s.d.Tracker != nil {
		s.d.Tracker.Track(token, iv)
	}
	return id, nil
}

func (s *Service) trackedLocked(token uint32, iv candles.Interval) bool {
	for _, lc := range s.liveCharts {
		if lc.token == token && lc.iv == iv {
			return true
		}
	}
	return false
}

// CloseLive releases a chart subscription; the builder stops tracking when no open chart shows (token, interval).
func (s *Service) CloseLive(_ context.Context, id string) error {
	s.chartMu.Lock()
	lc, ok := s.liveCharts[id]
	if !ok {
		s.chartMu.Unlock()
		return refused(chartStage, "not_found", "Unknown live subscription "+id+".")
	}
	delete(s.liveCharts, id)
	if !s.trackedLocked(lc.token, lc.iv) && s.d.Tracker != nil {
		s.d.Tracker.Untrack(lc.token, lc.iv)
	}
	s.chartMu.Unlock()
	s.d.Live.Release(ticker.ConsumerChart, id)
	return nil
}

// SearchInstruments is the symbol search (FR-WATCH-008 ranking is P0-2's); tags filter by instruments.Tag.
func (s *Service) SearchInstruments(_ context.Context, q string, tags []string, limit int) ([]InstrumentDTO, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []InstrumentDTO{}, nil
	}
	switch {
	case limit <= 0:
		limit = defaultSearch
	case limit > maxSearchLimit:
		limit = maxSearchLimit
	}
	want := map[instruments.Tag]bool{}
	for _, t := range tags {
		want[instruments.Tag(t)] = true
	}
	fetch := limit
	if len(want) > 0 {
		fetch = limit * 4
	}
	out := make([]InstrumentDTO, 0, limit)
	for _, r := range s.d.Instruments.Search(q, fetch) {
		if len(want) > 0 && !want[r.Instrument.Tag] {
			continue
		}
		d := instrumentDTO(r.Instrument)
		d.AlsoOnNSE = r.AlsoOnNSE
		out = append(out, d)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// InstrumentsByTokens resolves tokens; unknown tokens are omitted.
func (s *Service) InstrumentsByTokens(_ context.Context, tokens []uint32) ([]InstrumentDTO, error) {
	out := make([]InstrumentDTO, 0, len(tokens))
	for _, t := range tokens {
		if in, ok := s.d.Instruments.ByToken(t); ok {
			out = append(out, instrumentDTO(in))
		}
	}
	return out, nil
}

// InstrumentByIdentifier resolves "EXCHANGE:TRADINGSYMBOL".
func (s *Service) InstrumentByIdentifier(_ context.Context, id string) (InstrumentDTO, error) {
	in, ok := s.d.Instruments.ByID(strings.TrimSpace(id))
	if !ok {
		return InstrumentDTO{}, refused(instrumentsStage, "not_found", "No instrument "+id+" in the instrument list.")
	}
	return instrumentDTO(in), nil
}
