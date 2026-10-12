package kite_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestCanceledContextIsClassifiedCanceledNotNetwork(t *testing.T) { // Task 7 note: gokiteconnect takes no ctx
	e := newEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := e.cli.Orders(ctx)
	if k := kite.Classify(err); k != kite.KindCanceled || kite.IsOffline(err) {
		t.Fatalf("err = %v (kind %v), want canceled", err, k)
	}
	if n := e.srv.Calls(mock.RouteOrders); n != 0 {
		t.Fatalf("calls = %d, want 0 after cancellation", n)
	}
}

// A request already on the wire is aborted when ctx is cancelled (gokiteconnect takes no ctx;
// the client binds each attempt's requests to it) and reported as canceled, not offline.
func TestCancelWhileRequestInFlightIsCanceled(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(slow.Close)
	t.Cleanup(func() { close(release) })
	clk := clock.New(clock.Options{})
	lim := ratelimit.New(clk, ratelimit.Options{})
	lctx, lcancel := context.WithCancel(context.Background())
	t.Cleanup(lcancel)
	lim.Run(lctx)
	cli := kite.New(kite.Options{APIKey: "key", BaseURI: slow.URL, Limiter: lim, Clock: clk,
		Jitter: func() time.Duration { return time.Millisecond }})
	cli.SetAccessToken("token")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := cli.Orders(ctx); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if k := kite.Classify(err); k != kite.KindCanceled || kite.IsOffline(err) {
			t.Fatalf("err = %v (kind %v), want canceled", err, k)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Orders did not return after ctx was cancelled with the request in flight")
	}
}

func TestNewPanicsWithoutLimiterOrClock(t *testing.T) {
	clk := clock.New(clock.Options{})
	for name, o := range map[string]kite.Options{
		"Limiter": {Clock: clk},
		"Clock":   {Limiter: ratelimit.New(clk, ratelimit.Options{})},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if s, _ := r.(string); !strings.Contains(s, "Options."+name) {
					t.Fatalf("recover() = %v, want a panic naming Options.%s", r, name)
				}
			}()
			kite.New(o)
		})
	}
}
