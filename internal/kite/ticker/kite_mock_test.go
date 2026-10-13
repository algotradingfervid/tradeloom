package ticker_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite/mock"
	"tradeloom/internal/kite/ticker"
)

func waitFor(t *testing.T, within time.Duration, what string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !f() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Drives the real gokiteconnect ticker against the mock server: subscribe/mode frames, binary
// tick decoding, and — with SetReconnectMaxRetries(0) — the Manager's own reconnect.
func TestManagerOverRealTickerAndMockServer(t *testing.T) {
	srv := mock.NewServer()
	defer srv.Close()
	root := srv.TickerURL()
	m := ticker.New(ticker.Options{
		Dial:      ticker.KiteDialer("key", "token", &root),
		Clock:     clock.New(clock.Options{}),
		Bus:       bus.New(nil),
		BaseDelay: 100 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	waitFor(t, 3*time.Second, "connect", func() bool { return srv.Connects() == 1 })
	if err := m.Subscribe(ticker.ConsumerChart, "c", []uint32{738561}, ticker.ModeFull); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 3*time.Second, "subscribe frames", func() bool { return len(srv.Messages()) == 2 })
	msgs := srv.Messages()
	if msgs[0] != `{"a":"subscribe","v":[738561]}` || msgs[1] != `{"a":"mode","v":["full",[738561]]}` {
		t.Fatalf("frames %v", msgs)
	}
	raw, stop := m.Raw(8)
	defer stop()
	ts := time.Date(2026, 10, 12, 10, 0, 1, 0, clock.IST)
	srv.PushTicks(mock.Packet{Token: 738561, Mode: "full", LastPrice: 2450.05, Volume: 12345, Open: 2440, High: 2455, Low: 2431.5, Close: 2438, Timestamp: ts, LastTradeTime: ts})
	select {
	case tk := <-raw:
		if tk.LTP != 2450.05 || tk.Volume != 12345 || !tk.ExchangeTS.Equal(ts) || tk.Mode != "full" || tk.Close != 2438 {
			t.Fatalf("tick %+v", tk)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no tick")
	}

	// Drop the socket: the library notices within its 5 s data timeout, returns (max retries 0),
	// and the Manager redials and resubscribes.
	srv.DropConnections()
	waitFor(t, 15*time.Second, "reconnect", func() bool { return srv.Connects() == 2 })
	waitFor(t, 3*time.Second, "resubscribe", func() bool { return len(srv.Messages()) == 4 })
	if got := strings.Join(srv.Messages()[2:], " "); got != `{"a":"subscribe","v":[738561]} {"a":"mode","v":["full",[738561]]}` {
		t.Fatalf("resubscribe frames %s", got)
	}
}

// serveAsync runs c.Serve(ctx) and returns a channel closed when it returns.
func serveAsync(ctx context.Context, c ticker.Conn) <-chan struct{} {
	done := make(chan struct{})
	go func() { c.Serve(ctx); close(done) }()
	return done
}

func waitDone(t *testing.T, done <-chan struct{}, within time.Duration, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(within):
		t.Fatalf("Serve did not return after %s within %v", what, within)
	}
}

// connected dials a kite Conn against root and waits for OnConnect.
func connected(t *testing.T, ctx context.Context, root url.URL) (ticker.Conn, <-chan struct{}) {
	t.Helper()
	up := make(chan struct{}, 1)
	c := ticker.KiteDialer("key", "token", &root)(ticker.Events{OnConnect: func() { up <- struct{}{} }})
	done := serveAsync(ctx, c)
	select {
	case <-up:
	case <-time.After(3 * time.Second):
		t.Fatal("no OnConnect")
	}
	return c, done
}

// Stop before Serve (the race where the library's own Stop is a no-op until ServeWithContext has
// assigned its cancel): Serve must return at once without ever dialling.
func TestKiteConnStopBeforeServeNeverDials(t *testing.T) {
	srv := mock.NewServer()
	defer srv.Close()
	root := srv.TickerURL()
	c := ticker.KiteDialer("key", "token", &root)(ticker.Events{})
	c.Stop()
	c.Stop()
	waitDone(t, serveAsync(context.Background(), c), time.Second, "Stop-before-Serve")
	time.Sleep(50 * time.Millisecond)
	if n := srv.Connects(); n != 0 {
		t.Fatalf("connects after Stop-before-Serve = %d", n)
	}
}

// Stop on a live connection is non-blocking and idempotent, and ends Serve.
func TestKiteConnStopEndsServeAndIsIdempotent(t *testing.T) {
	srv := mock.NewServer()
	defer srv.Close()
	c, done := connected(t, context.Background(), srv.TickerURL())
	start := time.Now()
	c.Stop()
	c.Stop()
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Fatalf("Stop blocked for %v", d)
	}
	waitDone(t, done, 4*time.Second, "Stop")
	c.Stop()
	if err := c.Subscribe([]uint32{1}); err == nil {
		t.Fatal("Subscribe after Stop succeeded")
	}
}

// Cancelling Serve's ctx ends Serve even while the library's reader is blocked on the socket.
func TestKiteConnCtxCancelEndsServe(t *testing.T) {
	srv := mock.NewServer()
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	c, done := connected(t, ctx, srv.TickerURL())
	cancel()
	waitDone(t, done, 4*time.Second, "ctx cancel")
	c.Stop()
}

// A peer that stops reading must not block a write forever: the write deadline (~5 s) fails it,
// and the connection is torn down so the Manager redials and resubscribes.
func TestKiteConnWriteIsBoundedByDeadline(t *testing.T) {
	t.Parallel() // waits for the 5 s write deadline
	release := make(chan struct{})
	up := websocket.Upgrader{}
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		<-release // never read: the client's send buffers fill up
		_ = c.Close()
	}))
	defer hs.Close()
	defer close(release)
	u, _ := url.Parse(hs.URL)
	c, done := connected(t, context.Background(), url.URL{Scheme: "ws", Host: u.Host, Path: "/ws"})
	defer c.Stop()

	tokens := make([]uint32, 3000)
	for i := range tokens {
		tokens[i] = uint32(100000000 + i)
	}
	start := time.Now()
	var err error
	for i := 0; i < 100000 && err == nil && time.Since(start) < 20*time.Second; i++ {
		err = c.Subscribe(tokens)
	}
	if err == nil {
		t.Fatal("writes to a stalled peer never failed")
	}
	if d := time.Since(start); d > 9*time.Second {
		t.Fatalf("stalled write took %v (want ≈5 s deadline)", d)
	}
	waitDone(t, done, 4*time.Second, "failed write")
}

// The WebSocket URL carries api_key and access_token: errors handed to OnError (which the Manager
// logs) must never contain either.
func TestKiteConnErrorsNeverCarryCredentials(t *testing.T) {
	var mu sync.Mutex
	var errs []string
	root := url.URL{Scheme: "ws", Host: "[::1", Path: "/ws"} // url.Parse fails and echoes the full URL
	c := ticker.KiteDialer("my-api-key", "s3cret-access-token", &root)(ticker.Events{OnError: func(err error) {
		mu.Lock()
		errs = append(errs, err.Error())
		mu.Unlock()
	}})
	waitDone(t, serveAsync(context.Background(), c), 3*time.Second, "failed dial")
	mu.Lock()
	defer mu.Unlock()
	if len(errs) == 0 {
		t.Fatal("no error reported for a failed dial")
	}
	for _, e := range errs {
		if strings.Contains(e, "s3cret-access-token") || strings.Contains(e, "my-api-key") {
			t.Fatalf("error leaks credentials: %s", e)
		}
	}
}

// The Manager dials its three slots at once; the library writes gorilla's global DefaultDialer
// during every dial (ticker.go:300), so concurrent dials must be serialised (-race catches it).
func TestKiteConnsDialConcurrentlyWithoutRace(t *testing.T) {
	srv := mock.NewServer()
	defer srv.Close()
	root := srv.TickerURL()
	const n = 3
	var conns []ticker.Conn
	var dones []<-chan struct{}
	for i := 0; i < n; i++ {
		c := ticker.KiteDialer("key", "token", &root)(ticker.Events{})
		conns = append(conns, c)
		dones = append(dones, serveAsync(context.Background(), c))
	}
	waitFor(t, 5*time.Second, "three connects", func() bool { return srv.Connects() == n })
	for i, c := range conns {
		c.Stop()
		waitDone(t, dones[i], 4*time.Second, "Stop")
	}
}

// A Stop must not sit behind another conn's dial (up to the 7 s connect timeout): a conn waiting
// for the dial lock returns from Serve as soon as it is stopped or its ctx ends.
func TestKiteConnStopWhileWaitingForAnotherDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var accepted []net.Conn
	go func() {
		for {
			c, err := ln.Accept() // accept, never answer the handshake: the dial hangs
			if err != nil {
				return
			}
			mu.Lock()
			accepted = append(accepted, c)
			mu.Unlock()
		}
	}()
	defer func() {
		_ = ln.Close()
		mu.Lock()
		for _, c := range accepted {
			_ = c.Close()
		}
		mu.Unlock()
	}()
	root := url.URL{Scheme: "ws", Host: ln.Addr().String(), Path: "/ws"}
	a := ticker.KiteDialer("key", "token", &root)(ticker.Events{})
	aDone := serveAsync(context.Background(), a)
	defer func() { a.Stop(); <-aDone }()
	waitFor(t, 2*time.Second, "first dial in progress", func() bool { mu.Lock(); defer mu.Unlock(); return len(accepted) == 1 })

	b := ticker.KiteDialer("key", "token", &root)(ticker.Events{})
	bDone := serveAsync(context.Background(), b)
	time.Sleep(100 * time.Millisecond)
	b.Stop()
	waitDone(t, bDone, time.Second, "Stop while waiting to dial")

	ctx, cancel := context.WithCancel(context.Background())
	c := ticker.KiteDialer("key", "token", &root)(ticker.Events{})
	cDone := serveAsync(ctx, c)
	time.Sleep(100 * time.Millisecond)
	cancel()
	waitDone(t, cDone, time.Second, "ctx cancel while waiting to dial")
}

// The URL carries the query-escaped forms of the credentials; those must be redacted too.
func TestKiteConnErrorsNeverCarryEscapedCredentials(t *testing.T) {
	const key, tok = "api key+1", "tok/en+s3cret=="
	var mu sync.Mutex
	var errs []string
	root := url.URL{Scheme: "ws", Host: "[::1", Path: "/ws"}
	c := ticker.KiteDialer(key, tok, &root)(ticker.Events{OnError: func(err error) {
		mu.Lock()
		errs = append(errs, err.Error())
		mu.Unlock()
	}})
	waitDone(t, serveAsync(context.Background(), c), 3*time.Second, "failed dial")
	mu.Lock()
	defer mu.Unlock()
	if len(errs) == 0 {
		t.Fatal("no error reported for a failed dial")
	}
	for _, e := range errs {
		for _, s := range []string{key, tok, url.QueryEscape(key), url.QueryEscape(tok)} {
			if strings.Contains(e, s) {
				t.Fatalf("error leaks %q: %s", s, e)
			}
		}
	}
}

// Errors the library raises because we closed the socket ("use of closed network connection")
// are noise: none may reach OnError after Stop.
func TestKiteConnNoErrorsAfterStop(t *testing.T) {
	srv := mock.NewServer()
	defer srv.Close()
	var mu sync.Mutex
	var stopped bool
	var late []string
	up := make(chan struct{}, 1)
	root := srv.TickerURL()
	c := ticker.KiteDialer("key", "token", &root)(ticker.Events{
		OnConnect: func() { up <- struct{}{} },
		OnError: func(err error) {
			mu.Lock()
			if stopped {
				late = append(late, err.Error())
			}
			mu.Unlock()
		},
	})
	done := serveAsync(context.Background(), c)
	select {
	case <-up:
	case <-time.After(3 * time.Second):
		t.Fatal("no OnConnect")
	}
	mu.Lock()
	stopped = true
	mu.Unlock()
	c.Stop()
	waitDone(t, done, 4*time.Second, "Stop")
	mu.Lock()
	defer mu.Unlock()
	if len(late) > 0 {
		t.Fatalf("errors after Stop: %v", late)
	}
}

// When Serve returns on its own (peer dropped, library data timeout), the conn is closed: a later
// write reports not-connected instead of touching the dead socket.
func TestKiteConnWriteAfterServeReturnsIsNotConnected(t *testing.T) {
	t.Parallel() // waits for the library's 5 s data timeout
	srv := mock.NewServer()
	defer srv.Close()
	c, done := connected(t, context.Background(), srv.TickerURL())
	defer c.Stop()
	srv.DropConnections()
	waitDone(t, done, 10*time.Second, "peer drop")
	if err := c.Subscribe([]uint32{1}); err == nil || err.Error() != "ticker: not connected" {
		t.Fatalf("write after Serve returned: %v", err)
	}
}
