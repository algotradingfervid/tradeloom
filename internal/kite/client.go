package kite

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/logging"
)

// Options configures Client.
type Options struct {
	APIKey         string
	BaseURI        string             // "" = https://api.kite.trade; tests pass mock.Server.URL
	Limiter        *ratelimit.Limiter // required
	Clock          clock.Clock        // required
	DateSink       func(date, sent, received time.Time)
	OnSessionError func(e *Err)      // called after all queues are paused on a TokenException
	Transport      http.RoundTripper // nil = http.DefaultTransport (P0-1/P0-3 pass the allow-list transport)
	Jitter         func() time.Duration
	Log            *slog.Logger
}

// Client implements ReadClient and HistoricalPicker over gokiteconnect. Every attempt gets its
// own gokiteconnect client whose requests are bound to the call's ctx (gokiteconnect takes no
// context), so cancelling ctx aborts a request in flight. All of them share one recording
// transport; historical data uses a 30 s timeout, every other endpoint 10 s (§5.1). SetDebug is
// never enabled: it would log the Authorization header.
type Client struct {
	o     Options
	rec   *recorder
	mu    sync.RWMutex // guards token and o.APIKey (SetAPIKey, P0-3)
	token string
	// dumpMu serialises Instruments calls: the recorder keeps one captured dump error, so two
	// concurrent dumps could otherwise take each other's error.
	dumpMu sync.Mutex
}

var _ ReadClient = (*Client)(nil)
var _ HistoricalPicker = (*Client)(nil)

// New builds a Client. It makes no network call. It panics when Limiter or Clock is nil.
func New(o Options) *Client {
	if o.Limiter == nil {
		panic("kite.New: Options.Limiter is required")
	}
	if o.Clock == nil {
		panic("kite.New: Options.Clock is required")
	}
	if o.Transport == nil {
		o.Transport = http.DefaultTransport
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Jitter == nil {
		o.Jitter = func() time.Duration { return 500*time.Millisecond + rand.N(time.Second) }
	}
	return &Client{o: o, rec: &recorder{next: o.Transport, clk: o.Clock, sink: o.DateSink}}
}

// attempt builds the gokiteconnect client for one attempt: requests bound to ctx, 30 s timeout
// for historical data and 10 s otherwise (§5.1).
func (c *Client) attempt(ctx context.Context, class ratelimit.Class) *kiteconnect.Client {
	timeout := 10 * time.Second
	if class == ratelimit.ClassHistorical {
		timeout = 30 * time.Second
	}
	c.mu.RLock()
	key, token := c.o.APIKey, c.token
	c.mu.RUnlock()
	kc := kiteconnect.New(key)
	kc.SetHTTPClient(&http.Client{Timeout: timeout, Transport: ctxTransport{ctx: ctx, next: c.rec}})
	if c.o.BaseURI != "" {
		kc.SetBaseURI(c.o.BaseURI)
	}
	kc.SetAccessToken(token)
	return kc
}

// SetAccessToken installs the day's token (kite.Session, P0-3) for every later attempt.
// The token is registered with the log redactor so it can never appear in a log line.
func (c *Client) SetAccessToken(token string) {
	logging.RegisterSecret(token)
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
}

// APIKey returns the configured key.
func (c *Client) APIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.o.APIKey
}

// Call runs fn against a raw gokiteconnect client under the limiter and retry policy. It exists
// for kite.Session (GenerateSession, InvalidateAccessToken) in this package. Rule 3 stops other
// packages from naming *kiteconnect.Client, but that alone does not stop them using Call: a
// generic function instantiated by inference can receive the client without naming it. The
// G-0 archtest rules (Ruling 39) therefore forbid any reference to Call outside internal/kite.
func (c *Client) Call(ctx context.Context, class ratelimit.Class, idempotent bool, op string, fn func(kc *kiteconnect.Client) error) error {
	return c.call(ctx, class, idempotent, op, fn)
}

// call applies §5.3: limiter slot per attempt; 429 → class back-off then retry (≤ 3);
// network → retry idempotent calls ≤ 3 with 0.5–1.5 s jitter; server → one retry after 2 s;
// session → pause all queues, notify, no retry.
func (c *Client) call(ctx context.Context, class ratelimit.Class, idempotent bool, op string, fn func(kc *kiteconnect.Client) error) error {
	prio := ratelimit.PriorityFrom(ctx)
	// gokiteconnect takes no context: each attempt's client binds its requests to ctx (see
	// attempt), ctx is checked before each attempt, and a transport failure that coincides with
	// cancellation is reported as KindCanceled rather than as a network error.
	// A Kite answer (TokenException, 429, …) keeps its own kind so the session pause and the
	// class back-off still happen.
	locked := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn(c.attempt(ctx, class))
		if k := Classify(err); k == KindNetwork || k == KindUnknown {
			if cerr := ctx.Err(); cerr != nil && !errors.Is(err, errNothingPicked) {
				return cerr
			}
		}
		return err
	}
	netRetries, srvRetries, rlRetries := 0, 0, 0
	for {
		err := c.o.Limiter.Do(ctx, class, prio, locked)
		if errors.Is(err, errNothingPicked) {
			return err
		}
		if err == nil {
			c.o.Limiter.Success(class)
			return nil
		}
		if errors.Is(err, ratelimit.ErrStopped) || errors.Is(err, ratelimit.ErrBadClass) {
			c.o.Log.Info("kite call not admitted", "op", op, "class", class.String(), "err", err.Error())
			return wrap(op, err)
		}
		e := wrap(op, err)
		switch e.Kind {
		case KindSession:
			c.o.Limiter.PauseAll("session expired")
			if c.o.OnSessionError != nil {
				c.o.OnSessionError(e)
			}
			return e
		case KindRateLimited:
			c.o.Limiter.Backoff(class)
			if idempotent && rlRetries < 3 && ctx.Err() == nil {
				rlRetries++
				continue
			}
		case KindNetwork:
			if idempotent && netRetries < 3 {
				netRetries++
				if clock.Sleep(ctx, c.o.Clock, c.o.Jitter()) != nil {
					return wrap(op, ctx.Err())
				}
				continue
			}
		case KindServer:
			if idempotent && srvRetries < 1 {
				srvRetries++
				if clock.Sleep(ctx, c.o.Clock, 2*time.Second) != nil {
					return wrap(op, ctx.Err())
				}
				continue
			}
		}
		c.o.Log.Info("kite call failed", "op", op, "kind", e.Kind.String(), "code", e.Code, "type", e.Type)
		return e
	}
}

func get[T any](c *Client, ctx context.Context, class ratelimit.Class, op string, fn func(kc *kiteconnect.Client) (T, error)) (T, error) {
	var out T
	err := c.call(ctx, class, true, op, func(kc *kiteconnect.Client) error {
		v, err := fn(kc)
		if err != nil {
			return err
		}
		out = v
		return nil
	})
	return out, err
}

func (c *Client) Profile(ctx context.Context) (UserProfile, error) {
	return get(c, ctx, ratelimit.ClassOther, "Profile", (*kiteconnect.Client).GetUserProfile)
}

func (c *Client) Margins(ctx context.Context) (AllMargins, error) {
	return get(c, ctx, ratelimit.ClassOther, "Margins", (*kiteconnect.Client).GetUserMargins)
}

func (c *Client) Instruments(ctx context.Context) (Instruments, error) {
	return get(c, ctx, ratelimit.ClassOther, "Instruments", func(kc *kiteconnect.Client) (Instruments, error) {
		c.dumpMu.Lock()
		defer c.dumpMu.Unlock()
		c.rec.takeDumpErr()
		v, err := kc.GetInstruments()
		if de := c.rec.takeDumpErr(); de != nil {
			return nil, *de
		}
		return v, err
	})
}

// Historical sends from/to converted to IST: gokiteconnect formats them as wall time
// ("2006-01-02 15:04:05") in the value's own location.
func (c *Client) Historical(ctx context.Context, token int, interval string, from, to time.Time, continuous, oi bool) ([]HistoricalData, error) {
	return get(c, ctx, ratelimit.ClassHistorical, "Historical", func(kc *kiteconnect.Client) ([]HistoricalData, error) {
		return kc.GetHistoricalData(token, interval, from.In(clock.IST), to.In(clock.IST), continuous, oi)
	})
}

// HistoricalPick implements HistoricalPicker: the first attempt calls pick after the slot is
// granted; retries reuse the picked request.
func (c *Client) HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (HistoricalRequest, bool)) (HistoricalRequest, []HistoricalData, bool, error) {
	ctx = ratelimit.WithPriority(ctx, prio)
	var req HistoricalRequest
	picked := false
	var out []HistoricalData
	err := c.call(ctx, ratelimit.ClassHistorical, true, "Historical", func(kc *kiteconnect.Client) error {
		if !picked {
			r, ok := pick()
			if !ok {
				return errNothingPicked
			}
			req, picked = r, true
		}
		v, err := kc.GetHistoricalData(req.Token, req.Interval, req.From.In(clock.IST), req.To.In(clock.IST), req.Continuous, req.OI)
		out = v
		return err
	})
	if errors.Is(err, errNothingPicked) {
		return HistoricalRequest{}, nil, false, nil
	}
	return req, out, picked, err
}

type nothingPicked struct{}

func (nothingPicked) Error() string { return "nothing to fetch" }

var errNothingPicked error = nothingPicked{}

func (c *Client) Quote(ctx context.Context, ids ...string) (Quote, error) {
	return get(c, ctx, ratelimit.ClassQuote, "Quote", func(kc *kiteconnect.Client) (Quote, error) { return kc.GetQuote(ids...) })
}

func (c *Client) LTP(ctx context.Context, ids ...string) (QuoteLTP, error) {
	return get(c, ctx, ratelimit.ClassQuote, "LTP", func(kc *kiteconnect.Client) (QuoteLTP, error) { return kc.GetLTP(ids...) })
}

func (c *Client) OHLC(ctx context.Context, ids ...string) (QuoteOHLC, error) {
	return get(c, ctx, ratelimit.ClassQuote, "OHLC", func(kc *kiteconnect.Client) (QuoteOHLC, error) { return kc.GetOHLC(ids...) })
}

func (c *Client) Orders(ctx context.Context) (Orders, error) {
	return get(c, ctx, ratelimit.ClassOther, "Orders", (*kiteconnect.Client).GetOrders)
}

func (c *Client) OrderHistory(ctx context.Context, orderID string) ([]Order, error) {
	return get(c, ctx, ratelimit.ClassOther, "OrderHistory", func(kc *kiteconnect.Client) ([]Order, error) { return kc.GetOrderHistory(orderID) })
}

func (c *Client) Trades(ctx context.Context) (Trades, error) {
	return get(c, ctx, ratelimit.ClassOther, "Trades", (*kiteconnect.Client).GetTrades)
}

func (c *Client) OrderTrades(ctx context.Context, orderID string) ([]Trade, error) {
	return get(c, ctx, ratelimit.ClassOther, "OrderTrades", func(kc *kiteconnect.Client) ([]Trade, error) { return kc.GetOrderTrades(orderID) })
}

func (c *Client) Positions(ctx context.Context) (Positions, error) {
	return get(c, ctx, ratelimit.ClassOther, "Positions", (*kiteconnect.Client).GetPositions)
}

func (c *Client) Holdings(ctx context.Context) (Holdings, error) {
	return get(c, ctx, ratelimit.ClassOther, "Holdings", (*kiteconnect.Client).GetHoldings)
}

func (c *Client) AuctionInstruments(ctx context.Context) ([]AuctionInstrument, error) {
	return get(c, ctx, ratelimit.ClassOther, "AuctionInstruments", (*kiteconnect.Client).GetAuctionInstruments)
}

func (c *Client) GTTs(ctx context.Context) (GTTs, error) {
	return get(c, ctx, ratelimit.ClassOther, "GTTs", (*kiteconnect.Client).GetGTTs)
}

func (c *Client) GTT(ctx context.Context, id int) (GTT, error) {
	return get(c, ctx, ratelimit.ClassOther, "GTT", func(kc *kiteconnect.Client) (GTT, error) { return kc.GetGTT(id) })
}

func (c *Client) OrderMargins(ctx context.Context, p GetMarginParams) ([]OrderMargins, error) {
	return get(c, ctx, ratelimit.ClassOther, "OrderMargins", func(kc *kiteconnect.Client) ([]OrderMargins, error) { return kc.GetOrderMargins(p) })
}

func (c *Client) BasketMargins(ctx context.Context, p GetBasketParams) (BasketMargins, error) {
	return get(c, ctx, ratelimit.ClassOther, "BasketMargins", func(kc *kiteconnect.Client) (BasketMargins, error) { return kc.GetBasketMargins(p) })
}

func (c *Client) OrderCharges(ctx context.Context, p GetChargesParams) ([]OrderCharges, error) {
	return get(c, ctx, ratelimit.ClassOther, "OrderCharges", func(kc *kiteconnect.Client) ([]OrderCharges, error) { return kc.GetOrderCharges(p) })
}

func (c *Client) Alerts(ctx context.Context, filters map[string]string) ([]Alert, error) {
	return get(c, ctx, ratelimit.ClassOther, "Alerts", func(kc *kiteconnect.Client) ([]Alert, error) { return kc.GetAlerts(filters) })
}

func (c *Client) AlertHistory(ctx context.Context, uuid string) ([]AlertHistory, error) {
	return get(c, ctx, ratelimit.ClassOther, "AlertHistory", func(kc *kiteconnect.Client) ([]AlertHistory, error) { return kc.GetAlertHistory(uuid) })
}
