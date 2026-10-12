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

// Client implements ReadClient and HistoricalPicker over two gokiteconnect clients that share
// one recording transport: a 10 s client for every endpoint and a 30 s client for historical
// data (§5.1). SetDebug is never enabled: it would log the Authorization header.
type Client struct {
	o    Options
	rec  *recorder
	api  *kiteconnect.Client
	hist *kiteconnect.Client
	mu   sync.RWMutex // gokiteconnect reads the token unsynchronised: calls hold RLock, SetAccessToken Lock
	// dumpMu serialises Instruments calls: the recorder keeps one captured dump error, so two
	// concurrent dumps could otherwise take each other's error.
	dumpMu sync.Mutex
}

var _ ReadClient = (*Client)(nil)
var _ HistoricalPicker = (*Client)(nil)

// New builds a Client. It makes no network call.
func New(o Options) *Client {
	if o.Transport == nil {
		o.Transport = http.DefaultTransport
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.Jitter == nil {
		o.Jitter = func() time.Duration { return 500*time.Millisecond + rand.N(time.Second) }
	}
	rec := &recorder{next: o.Transport, clk: o.Clock, sink: o.DateSink}
	mk := func(timeout time.Duration) *kiteconnect.Client {
		kc := kiteconnect.New(o.APIKey)
		kc.SetHTTPClient(&http.Client{Timeout: timeout, Transport: rec})
		if o.BaseURI != "" {
			kc.SetBaseURI(o.BaseURI)
		}
		return kc
	}
	return &Client{o: o, rec: rec, api: mk(10 * time.Second), hist: mk(30 * time.Second)}
}

// SetAccessToken installs the day's token (kite.Session, P0-3) on both inner clients.
// The token is registered with the log redactor so it can never appear in a log line.
func (c *Client) SetAccessToken(token string) {
	logging.RegisterSecret(token)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.api.SetAccessToken(token)
	c.hist.SetAccessToken(token)
}

// APIKey returns the configured key.
func (c *Client) APIKey() string { return c.o.APIKey }

// Call runs fn against the 10 s client under the limiter and retry policy. It exists for
// kite.Session (GenerateSession, InvalidateAccessToken) in this package; packages outside
// internal/kite cannot name *kiteconnect.Client, so they cannot use it (rule 3).
func (c *Client) Call(ctx context.Context, class ratelimit.Class, idempotent bool, op string, fn func(kc *kiteconnect.Client) error) error {
	return c.call(ctx, class, idempotent, op, func() error { return fn(c.api) })
}

// call applies §5.3: limiter slot per attempt; 429 → class back-off then retry (≤ 3);
// network → retry idempotent calls ≤ 3 with 0.5–1.5 s jitter; server → one retry after 2 s;
// session → pause all queues, notify, no retry.
func (c *Client) call(ctx context.Context, class ratelimit.Class, idempotent bool, op string, fn func() error) error {
	prio := ratelimit.PriorityFrom(ctx)
	// gokiteconnect takes no context: check ctx before each attempt, and report a transport
	// failure that coincides with cancellation as KindCanceled rather than as a network error.
	// A Kite answer (TokenException, 429, …) keeps its own kind so the session pause and the
	// class back-off still happen.
	locked := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := func() error {
			c.mu.RLock()
			defer c.mu.RUnlock()
			return fn()
		}()
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

func get[T any](c *Client, ctx context.Context, class ratelimit.Class, op string, fn func() (T, error)) (T, error) {
	var out T
	err := c.call(ctx, class, true, op, func() error {
		v, err := fn()
		if err != nil {
			return err
		}
		out = v
		return nil
	})
	return out, err
}

func (c *Client) Profile(ctx context.Context) (UserProfile, error) {
	return get(c, ctx, ratelimit.ClassOther, "Profile", c.api.GetUserProfile)
}

func (c *Client) Margins(ctx context.Context) (AllMargins, error) {
	return get(c, ctx, ratelimit.ClassOther, "Margins", c.api.GetUserMargins)
}

func (c *Client) Instruments(ctx context.Context) (Instruments, error) {
	return get(c, ctx, ratelimit.ClassOther, "Instruments", func() (Instruments, error) {
		c.dumpMu.Lock()
		defer c.dumpMu.Unlock()
		c.rec.takeDumpErr()
		v, err := c.api.GetInstruments()
		if de := c.rec.takeDumpErr(); de != nil {
			return nil, *de
		}
		return v, err
	})
}

// Historical sends from/to converted to IST: gokiteconnect formats them as wall time
// ("2006-01-02 15:04:05") in the value's own location.
func (c *Client) Historical(ctx context.Context, token int, interval string, from, to time.Time, continuous, oi bool) ([]HistoricalData, error) {
	return get(c, ctx, ratelimit.ClassHistorical, "Historical", func() ([]HistoricalData, error) {
		return c.hist.GetHistoricalData(token, interval, from.In(clock.IST), to.In(clock.IST), continuous, oi)
	})
}

// HistoricalPick implements HistoricalPicker: the first attempt calls pick after the slot is
// granted; retries reuse the picked request.
func (c *Client) HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (HistoricalRequest, bool)) (HistoricalRequest, []HistoricalData, bool, error) {
	ctx = ratelimit.WithPriority(ctx, prio)
	var req HistoricalRequest
	picked := false
	var out []HistoricalData
	err := c.call(ctx, ratelimit.ClassHistorical, true, "Historical", func() error {
		if !picked {
			r, ok := pick()
			if !ok {
				return errNothingPicked
			}
			req, picked = r, true
		}
		v, err := c.hist.GetHistoricalData(req.Token, req.Interval, req.From.In(clock.IST), req.To.In(clock.IST), req.Continuous, req.OI)
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
	return get(c, ctx, ratelimit.ClassQuote, "Quote", func() (Quote, error) { return c.api.GetQuote(ids...) })
}

func (c *Client) LTP(ctx context.Context, ids ...string) (QuoteLTP, error) {
	return get(c, ctx, ratelimit.ClassQuote, "LTP", func() (QuoteLTP, error) { return c.api.GetLTP(ids...) })
}

func (c *Client) OHLC(ctx context.Context, ids ...string) (QuoteOHLC, error) {
	return get(c, ctx, ratelimit.ClassQuote, "OHLC", func() (QuoteOHLC, error) { return c.api.GetOHLC(ids...) })
}

func (c *Client) Orders(ctx context.Context) (Orders, error) {
	return get(c, ctx, ratelimit.ClassOther, "Orders", c.api.GetOrders)
}

func (c *Client) OrderHistory(ctx context.Context, orderID string) ([]Order, error) {
	return get(c, ctx, ratelimit.ClassOther, "OrderHistory", func() ([]Order, error) { return c.api.GetOrderHistory(orderID) })
}

func (c *Client) Trades(ctx context.Context) (Trades, error) {
	return get(c, ctx, ratelimit.ClassOther, "Trades", c.api.GetTrades)
}

func (c *Client) OrderTrades(ctx context.Context, orderID string) ([]Trade, error) {
	return get(c, ctx, ratelimit.ClassOther, "OrderTrades", func() ([]Trade, error) { return c.api.GetOrderTrades(orderID) })
}

func (c *Client) Positions(ctx context.Context) (Positions, error) {
	return get(c, ctx, ratelimit.ClassOther, "Positions", c.api.GetPositions)
}

func (c *Client) Holdings(ctx context.Context) (Holdings, error) {
	return get(c, ctx, ratelimit.ClassOther, "Holdings", c.api.GetHoldings)
}

func (c *Client) AuctionInstruments(ctx context.Context) ([]AuctionInstrument, error) {
	return get(c, ctx, ratelimit.ClassOther, "AuctionInstruments", c.api.GetAuctionInstruments)
}

func (c *Client) GTTs(ctx context.Context) (GTTs, error) {
	return get(c, ctx, ratelimit.ClassOther, "GTTs", c.api.GetGTTs)
}

func (c *Client) GTT(ctx context.Context, id int) (GTT, error) {
	return get(c, ctx, ratelimit.ClassOther, "GTT", func() (GTT, error) { return c.api.GetGTT(id) })
}

func (c *Client) OrderMargins(ctx context.Context, p GetMarginParams) ([]OrderMargins, error) {
	return get(c, ctx, ratelimit.ClassOther, "OrderMargins", func() ([]OrderMargins, error) { return c.api.GetOrderMargins(p) })
}

func (c *Client) BasketMargins(ctx context.Context, p GetBasketParams) (BasketMargins, error) {
	return get(c, ctx, ratelimit.ClassOther, "BasketMargins", func() (BasketMargins, error) { return c.api.GetBasketMargins(p) })
}

func (c *Client) OrderCharges(ctx context.Context, p GetChargesParams) ([]OrderCharges, error) {
	return get(c, ctx, ratelimit.ClassOther, "OrderCharges", func() ([]OrderCharges, error) { return c.api.GetOrderCharges(p) })
}

func (c *Client) Alerts(ctx context.Context, filters map[string]string) ([]Alert, error) {
	return get(c, ctx, ratelimit.ClassOther, "Alerts", func() ([]Alert, error) { return c.api.GetAlerts(filters) })
}

func (c *Client) AlertHistory(ctx context.Context, uuid string) ([]AlertHistory, error) {
	return get(c, ctx, ratelimit.ClassOther, "AlertHistory", func() ([]AlertHistory, error) { return c.api.GetAlertHistory(uuid) })
}
