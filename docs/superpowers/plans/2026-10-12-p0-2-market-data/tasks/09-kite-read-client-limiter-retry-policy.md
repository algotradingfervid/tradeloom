> **P0-2 Time & Market Data Implementation Plan — Task 9.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 9: Kite read client: limiter, retry policy, Date header and dump-error capture

**Files:**
- Create: `internal/kite/transport.go`
- Create: `internal/kite/client.go`
- Test: `internal/kite/client_test.go`

**Interfaces:**
- Consumes: Task 1/2: `clock.Clock`, `clock.New` (real clock in tests), `clock.Sleep`, `(*clock.Real).ObserveHTTPDate` (as `DateSink`). Task 6: `ratelimit.New`, `Limiter.Do/Backoff/Success/PauseAll`, `WithPriority`, `PriorityFrom`, classes. Task 7: `ReadClient`, `HistoricalPicker`, `Err`, `Classify`, `wrap`. Task 8: `mock.NewServer`, `Failure`, routes, `SetCandles`, `SetDate`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
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
type Client struct{ /* unexported fields */ }
func New(o Options) *Client
func (c *Client) SetAccessToken(token string)
func (c *Client) APIKey() string
func (c *Client) Call(ctx context.Context, class ratelimit.Class, idempotent bool, op string, fn func(kc *kiteconnect.Client) error) error
func (c *Client) Profile(ctx context.Context) (UserProfile, error)
func (c *Client) Margins(ctx context.Context) (AllMargins, error)
func (c *Client) Instruments(ctx context.Context) (Instruments, error)
func (c *Client) Historical(ctx context.Context, token int, interval string, from, to time.Time, continuous, oi bool) ([]HistoricalData, error)
func (c *Client) HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (HistoricalRequest, bool)) (HistoricalRequest, []HistoricalData, bool, error)
func (nothingPicked) Error() string
func (c *Client) Quote(ctx context.Context, ids ...string) (Quote, error)
func (c *Client) LTP(ctx context.Context, ids ...string) (QuoteLTP, error)
func (c *Client) OHLC(ctx context.Context, ids ...string) (QuoteOHLC, error)
func (c *Client) Orders(ctx context.Context) (Orders, error)
func (c *Client) OrderHistory(ctx context.Context, orderID string) ([]Order, error)
func (c *Client) Trades(ctx context.Context) (Trades, error)
func (c *Client) OrderTrades(ctx context.Context, orderID string) ([]Trade, error)
func (c *Client) Positions(ctx context.Context) (Positions, error)
func (c *Client) Holdings(ctx context.Context) (Holdings, error)
func (c *Client) AuctionInstruments(ctx context.Context) ([]AuctionInstrument, error)
func (c *Client) GTTs(ctx context.Context) (GTTs, error)
func (c *Client) GTT(ctx context.Context, id int) (GTT, error)
func (c *Client) OrderMargins(ctx context.Context, p GetMarginParams) ([]OrderMargins, error)
func (c *Client) BasketMargins(ctx context.Context, p GetBasketParams) (BasketMargins, error)
func (c *Client) OrderCharges(ctx context.Context, p GetChargesParams) ([]OrderCharges, error)
func (c *Client) Alerts(ctx context.Context, filters map[string]string) ([]Alert, error)
func (c *Client) AlertHistory(ctx context.Context, uuid string) ([]AlertHistory, error)
```

FR-DATA-003 error policy end to end against the mock: 429 isolated to its class (AC2), `TokenException` pauses everything and notifies P0-3 (AC4), non-idempotent calls never retried blindly (AC6, client side), HTTP `Date` reaches the app clock (FR-DATA-004 AC4), the read client never touches order-write routes (G-0).

- [ ] **Step 1: Write the failing tests**

`internal/kite/client_test.go`:

```go
package kite_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/mock"
	"tradeloom/internal/kite/ratelimit"
)

type env struct {
	srv   *mock.Server
	lim   *ratelimit.Limiter
	cli   *kite.Client
	mu    sync.Mutex
	sess  []*kite.Err
	dates []time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{srv: mock.NewServer()}
	t.Cleanup(e.srv.Close)
	clk := clock.New(clock.Options{})
	e.lim = ratelimit.New(clk, ratelimit.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	e.lim.Run(ctx)
	e.cli = kite.New(kite.Options{
		APIKey: "key", BaseURI: e.srv.URL, Limiter: e.lim, Clock: clk,
		Jitter:         func() time.Duration { return time.Millisecond },
		OnSessionError: func(x *kite.Err) { e.mu.Lock(); e.sess = append(e.sess, x); e.mu.Unlock() },
		DateSink:       func(d, s, r time.Time) { e.mu.Lock(); e.dates = append(e.dates, d); e.mu.Unlock() },
	})
	e.cli.SetAccessToken("token")
	return e
}

func TestHistoricalSendsISTWallTimeAndParsesCandles(t *testing.T) {
	e := newEnv(t)
	day := time.Date(2026, 10, 9, 9, 15, 0, 0, clock.IST)
	e.srv.SetCandles(408065, "minute", []mock.Candle{{T: day, Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 100, OI: 7}})
	from := time.Date(2026, 10, 9, 3, 45, 0, 0, time.UTC) // 09:15 IST
	got, err := e.cli.Historical(context.Background(), 408065, "minute", from, from.Add(time.Hour), false, true)
	if err != nil {
		t.Fatal(err)
	}
	req := e.srv.HistoricalRequests()[0]
	if req.From != "2026-10-09 09:15:00" || req.OI != "1" {
		t.Fatalf("sent %+v, want IST wall time and oi=1", req)
	}
	if len(got) != 1 || got[0].Close != 1.5 || got[0].OI != 7 || !got[0].Date.Equal(day) {
		t.Fatalf("got %+v", got)
	}
}

func TestRateLimitedHistoricalBacksOffThatClassOnly(t *testing.T) { // FR-DATA-003 AC2 end to end
	e := newEnv(t)
	e.srv.FailNext(mock.RouteHistorical, mock.Failure{Status: http.StatusTooManyRequests, ErrorType: "NetworkException", Message: "Too many requests"})
	done := make(chan time.Duration, 1)
	start := time.Now()
	go func() {
		_, err := e.cli.Historical(context.Background(), 1, "day", start, start, false, false)
		if err != nil {
			t.Error(err)
		}
		done <- time.Since(start)
	}()
	time.Sleep(100 * time.Millisecond)
	qs := time.Now()
	if _, err := e.cli.LTP(context.Background(), "NSE:INFY"); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(qs); d > 500*time.Millisecond {
		t.Fatalf("quote waited %v behind a historical back-off", d)
	}
	if d := <-done; d < time.Second {
		t.Fatalf("historical retry after %v, want ≥ 1 s", d)
	}
	if e.srv.Calls(mock.RouteHistorical) != 2 {
		t.Fatalf("historical calls = %d, want 2 (429 + retry)", e.srv.Calls(mock.RouteHistorical))
	}
	if n := e.lim.Snapshot().Classes[ratelimit.ClassHistorical].HTTP429; n != 1 {
		t.Fatalf("http429 = %d", n)
	}
}

func TestTokenExceptionPausesAllQueuesAndNotifies(t *testing.T) { // FR-DATA-003 AC4 (backend)
	e := newEnv(t)
	e.srv.FailNext(mock.RouteProfile, mock.Failure{Status: 403, ErrorType: "TokenException", Message: "Incorrect `api_key` or `access_token`."})
	_, err := e.cli.Profile(context.Background())
	if kite.Classify(err) != kite.KindSession {
		t.Fatalf("err = %v", err)
	}
	if e.srv.Calls(mock.RouteProfile) != 1 {
		t.Fatal("TokenException must not be retried")
	}
	if !e.lim.Snapshot().Paused {
		t.Fatal("queues not paused")
	}
	e.mu.Lock()
	n := len(e.sess)
	e.mu.Unlock()
	if n != 1 {
		t.Fatalf("OnSessionError calls = %d", n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := e.cli.LTP(ctx, "NSE:INFY"); kite.Classify(err) != kite.KindCanceled {
		t.Fatalf("paused quote call returned %v, want to block until ctx ends", err)
	}
}

func TestInstrumentsTokenExceptionIsClassifiedNotCSVError(t *testing.T) {
	e := newEnv(t)
	e.srv.FailNext(mock.RouteInstruments, mock.Failure{Status: 403, ErrorType: "TokenException", Message: "expired"})
	_, err := e.cli.Instruments(context.Background())
	if kite.Classify(err) != kite.KindSession {
		t.Fatalf("err = %v (kind %v), want session", err, kite.Classify(err))
	}
}

func TestNetworkErrorsRetriedThreeTimes(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 4; i++ {
		e.srv.FailNext(mock.RouteOrders, mock.Failure{Status: 503, ErrorType: "NetworkException", Message: "gateway"})
	}
	_, err := e.cli.Orders(context.Background())
	if kite.Classify(err) != kite.KindNetwork || !kite.IsOffline(err) {
		t.Fatalf("err = %v", err)
	}
	if n := e.srv.Calls(mock.RouteOrders); n != 4 {
		t.Fatalf("calls = %d, want 1 + 3 retries", n)
	}
}

func TestServerErrorRetriedOnceAfterTwoSeconds(t *testing.T) {
	e := newEnv(t)
	e.srv.FailNext(mock.RouteOrders, mock.Failure{Status: 500, ErrorType: "GeneralException", Message: "oops"})
	start := time.Now()
	if _, err := e.cli.Orders(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < 2*time.Second {
		t.Fatalf("retried after %v, want ≥ 2 s", d)
	}
}

func TestDateHeaderFeedsClock(t *testing.T) {
	e := newEnv(t)
	fixed := time.Date(2026, 10, 12, 4, 30, 0, 0, time.UTC)
	e.srv.SetDate(func() time.Time { return fixed })
	if _, err := e.cli.Profile(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.dates) == 0 || !e.dates[len(e.dates)-1].Equal(fixed) {
		t.Fatalf("dates = %v", e.dates)
	}
}

func TestReadClientNeverCallsOrderWriteRoutes(t *testing.T) { // G-0 (§2.5 check 3, P0-2 part)
	e := newEnv(t)
	ctx := context.Background()
	var rc kite.ReadClient = e.cli
	_, _ = rc.Profile(ctx)
	_, _ = rc.Orders(ctx)
	_, _ = rc.LTP(ctx, "NSE:INFY")
	_, _ = rc.Instruments(ctx)
	if n := e.srv.Calls(mock.RouteOrderWrite); n != 0 {
		t.Fatalf("order-write calls = %d", n)
	}
}

func TestNonIdempotentCallsAreNeverRetried(t *testing.T) { // FR-DATA-003 AC6 (client side)
	e := newEnv(t)
	e.srv.FailNext(mock.RouteOrders, mock.Failure{Status: 503, ErrorType: "NetworkException", Message: "gateway"})
	err := e.cli.Call(context.Background(), ratelimit.ClassOther, false, "Orders", func(kc *kiteconnect.Client) error {
		_, err := kc.GetOrders()
		return err
	})
	if kite.Classify(err) != kite.KindNetwork {
		t.Fatalf("err = %v", err)
	}
	if n := e.srv.Calls(mock.RouteOrders); n != 1 {
		t.Fatalf("calls = %d, want 1 (no blind retry)", n)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/`
Expected: the build fails — output contains `undefined: kite.New` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/kite/transport.go`:

```go
package kite

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/clock"
)

// recorder is the RoundTripper under both gokiteconnect clients. It (a) hands every response's
// Date header to the app clock (FR-DATA-004) and (b) remembers the error envelope of a failed
// instrument dump, because gokiteconnect's GetInstruments does not check the HTTP status
// (market.go parseInstruments) and would otherwise report a CSV parse error.
type recorder struct {
	next http.RoundTripper
	clk  clock.Clock
	sink func(date, sent, received time.Time)

	mu      sync.Mutex
	dumpErr *kiteconnect.Error
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	sent := r.clk.System()
	resp, err := r.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	received := r.clk.System()
	if r.sink != nil {
		if d, perr := http.ParseTime(resp.Header.Get("Date")); perr == nil {
			r.sink(d, sent, received)
		}
	}
	if isDump(req.URL.Path) && resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		var env struct {
			ErrorType string `json:"error_type"`
			Message   string `json:"message"`
		}
		_ = json.Unmarshal(body, &env)
		if env.ErrorType == "" {
			env.ErrorType = kiteconnect.GetErrorName(resp.StatusCode)
		}
		if env.Message == "" {
			env.Message = http.StatusText(resp.StatusCode)
		}
		r.mu.Lock()
		r.dumpErr = &kiteconnect.Error{Code: resp.StatusCode, ErrorType: env.ErrorType, Message: env.Message}
		r.mu.Unlock()
	}
	return resp, nil
}

func isDump(path string) bool {
	return path == "/instruments" || (strings.HasPrefix(path, "/instruments/") && !strings.HasPrefix(path, "/instruments/historical/"))
}

// takeDumpErr returns and clears the last recorded dump failure.
func (r *recorder) takeDumpErr() *kiteconnect.Error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.dumpErr
	r.dumpErr = nil
	return e
}
```

`internal/kite/client.go`:

```go
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
func (c *Client) SetAccessToken(token string) {
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
	locked := func() error {
		c.mu.RLock()
		defer c.mu.RUnlock()
		return fn()
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
			if idempotent && rlRetries < 3 {
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/`
Expected: `ok  	tradeloom/internal/kite`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/kite/transport.go \
  internal/kite/client.go \
  internal/kite/client_test.go
git commit -m "$(cat <<'EOF'
feat(kite): read client with shared limiter, retry policy and clock sampling

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
