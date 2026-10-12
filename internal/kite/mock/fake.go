package mock

import (
	"context"
	"sort"
	"sync"
	"time"

	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
)

// Fake is an in-memory kite.ReadClient and kite.HistoricalPicker. Unscripted methods return
// zero values. Every call is counted by method name.
type Fake struct {
	mu sync.Mutex
	// HistoricalFunc answers Historical and HistoricalPick; nil returns no candles.
	HistoricalFunc func(req kite.HistoricalRequest) ([]kite.HistoricalData, error)
	// InstrumentsFunc answers Instruments; nil returns an empty dump.
	InstrumentsFunc func() (kite.Instruments, error)
	// Limiter, when set, makes HistoricalPick wait for a historical slot like the real client.
	Limiter *ratelimit.Limiter
	calls   map[string]int
	hist    []kite.HistoricalRequest
}

var _ kite.ReadClient = (*Fake)(nil)
var _ kite.HistoricalPicker = (*Fake)(nil)

// NewFake returns an empty Fake.
func NewFake() *Fake { return &Fake{calls: map[string]int{}} }

func (f *Fake) count(m string) { f.mu.Lock(); f.calls[m]++; f.mu.Unlock() }

// Calls returns how many times method m was called.
func (f *Fake) Calls(m string) int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls[m] }

// HistoricalRequests returns historical requests in call order.
func (f *Fake) HistoricalRequests() []kite.HistoricalRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]kite.HistoricalRequest(nil), f.hist...)
}

func (f *Fake) historical(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
	f.mu.Lock()
	f.calls["Historical"]++
	f.hist = append(f.hist, req)
	fn := f.HistoricalFunc
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(req)
}

func (f *Fake) Historical(ctx context.Context, token int, iv string, from, to time.Time, continuous, oi bool) ([]kite.HistoricalData, error) {
	return f.historical(kite.HistoricalRequest{Token: token, Interval: iv, From: from, To: to, Continuous: continuous, OI: oi})
}

func (f *Fake) HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (kite.HistoricalRequest, bool)) (kite.HistoricalRequest, []kite.HistoricalData, bool, error) {
	if f.Limiter != nil {
		if err := f.Limiter.Acquire(ctx, ratelimit.ClassHistorical, prio); err != nil {
			return kite.HistoricalRequest{}, nil, false, err
		}
	}
	req, ok := pick()
	if !ok {
		return kite.HistoricalRequest{}, nil, false, nil
	}
	data, err := f.historical(req)
	return req, data, true, err
}

func (f *Fake) Instruments(ctx context.Context) (kite.Instruments, error) {
	f.count("Instruments")
	if f.InstrumentsFunc == nil {
		return nil, nil
	}
	return f.InstrumentsFunc()
}

func (f *Fake) Profile(ctx context.Context) (kite.UserProfile, error) {
	f.count("Profile")
	return kite.UserProfile{}, nil
}
func (f *Fake) Margins(ctx context.Context) (kite.AllMargins, error) {
	f.count("Margins")
	return kite.AllMargins{}, nil
}
func (f *Fake) Quote(ctx context.Context, ids ...string) (kite.Quote, error) {
	f.count("Quote")
	return kite.Quote{}, nil
}
func (f *Fake) LTP(ctx context.Context, ids ...string) (kite.QuoteLTP, error) {
	f.count("LTP")
	return kite.QuoteLTP{}, nil
}
func (f *Fake) OHLC(ctx context.Context, ids ...string) (kite.QuoteOHLC, error) {
	f.count("OHLC")
	return kite.QuoteOHLC{}, nil
}
func (f *Fake) Orders(ctx context.Context) (kite.Orders, error) { f.count("Orders"); return nil, nil }
func (f *Fake) OrderHistory(ctx context.Context, id string) ([]kite.Order, error) {
	f.count("OrderHistory")
	return nil, nil
}
func (f *Fake) Trades(ctx context.Context) (kite.Trades, error) { f.count("Trades"); return nil, nil }
func (f *Fake) OrderTrades(ctx context.Context, id string) ([]kite.Trade, error) {
	f.count("OrderTrades")
	return nil, nil
}
func (f *Fake) Positions(ctx context.Context) (kite.Positions, error) {
	f.count("Positions")
	return kite.Positions{}, nil
}
func (f *Fake) Holdings(ctx context.Context) (kite.Holdings, error) {
	f.count("Holdings")
	return nil, nil
}
func (f *Fake) AuctionInstruments(ctx context.Context) ([]kite.AuctionInstrument, error) {
	f.count("AuctionInstruments")
	return nil, nil
}
func (f *Fake) GTTs(ctx context.Context) (kite.GTTs, error) { f.count("GTTs"); return nil, nil }
func (f *Fake) GTT(ctx context.Context, id int) (kite.GTT, error) {
	f.count("GTT")
	return kite.GTT{}, nil
}
func (f *Fake) OrderMargins(ctx context.Context, p kite.GetMarginParams) ([]kite.OrderMargins, error) {
	f.count("OrderMargins")
	return nil, nil
}
func (f *Fake) BasketMargins(ctx context.Context, p kite.GetBasketParams) (kite.BasketMargins, error) {
	f.count("BasketMargins")
	return kite.BasketMargins{}, nil
}
func (f *Fake) OrderCharges(ctx context.Context, p kite.GetChargesParams) ([]kite.OrderCharges, error) {
	f.count("OrderCharges")
	return nil, nil
}
func (f *Fake) Alerts(ctx context.Context, filters map[string]string) ([]kite.Alert, error) {
	f.count("Alerts")
	return nil, nil
}
func (f *Fake) AlertHistory(ctx context.Context, uuid string) ([]kite.AlertHistory, error) {
	f.count("AlertHistory")
	return nil, nil
}

// Methods returns every method name called so far, sorted (G-0 check helper).
func (f *Fake) Methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for m := range f.calls {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
