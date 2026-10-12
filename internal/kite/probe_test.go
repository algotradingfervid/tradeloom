package kite

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/clock"
)

func TestIsNoDataPermission_Variants(t *testing.T) {
	yes := []error{
		APIError("PermissionException", 403, "Insufficient permission for that call."),
		APIError(kiteconnect.PermissionError, 403, "Insufficient permission"),
		APIError(kiteconnect.InputError, 403, "Not enabled for this app"),
		APIError(kiteconnect.GeneralError, 400, "Insufficient permission for that call."),
		fmt.Errorf("historical: %w", APIError("PermissionException", 403, "x")),
	}
	no := []error{
		nil,
		APIError(kiteconnect.TokenError, 403, "Incorrect `api_key` or `access_token`."),
		APIError(kiteconnect.NetworkError, 503, "Request failed."),
		APIError(kiteconnect.InputError, 400, "invalid from date"),
		errors.New("dial tcp 1.2.3.4:443: connect: connection refused"),
	}
	for _, e := range yes {
		if !IsNoDataPermission(e) {
			t.Errorf("IsNoDataPermission(%v) = false", e)
		}
	}
	for _, e := range no {
		if IsNoDataPermission(e) {
			t.Errorf("IsNoDataPermission(%v) = true", e)
		}
	}
}

type probeCalls struct {
	mu     sync.Mutex
	ticker int
}

func newTestProbe(histErr, tickErr error, calls *probeCalls) *CapabilityProbe {
	return &CapabilityProbe{
		Historical: func(context.Context) error { return histErr },
		TickerConnect: func(context.Context, string, string) error {
			calls.mu.Lock()
			calls.ticker++
			calls.mu.Unlock()
			return tickErr
		},
		Log: slog.Default(),
	}
}

func TestProbe_ConnectWhenHistoricalOK(t *testing.T) {
	c := &probeCalls{}
	if got := newTestProbe(nil, nil, c).Probe(context.Background(), "k", "t"); got != PlanConnect || c.ticker != 0 {
		t.Fatalf("plan %s ticker calls %d", got, c.ticker)
	}
}

// FR-ACCT-007 AC1 and Review Focus 4.
func TestProbe_PersonalPlan(t *testing.T) {
	c := &probeCalls{}
	p := newTestProbe(APIError("PermissionException", 403, "Insufficient permission for that call."), nil, c)
	if got := p.Probe(context.Background(), "k", "t"); got != PlanNone || c.ticker != 0 {
		t.Fatalf("plan %s ticker calls %d", got, c.ticker)
	}
}

func TestProbe_AmbiguousHistoricalFallsBackToTicker(t *testing.T) {
	netErr := APIError(kiteconnect.NetworkError, 503, "Request failed.")
	if got := newTestProbe(netErr, nil, &probeCalls{}).Probe(context.Background(), "k", "t"); got != PlanConnect {
		t.Fatalf("ticker ok -> %s", got)
	}
	if got := newTestProbe(netErr, errors.New("bad handshake"), &probeCalls{}).Probe(context.Background(), "k", "t"); got != PlanUnknown {
		t.Fatalf("ticker failed -> %s", got)
	}
}

func wsURL(t *testing.T, srv *httptest.Server) *url.URL {
	t.Helper()
	u, err := url.Parse(strings.Replace(srv.URL, "http://", "ws://", 1))
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestDialTicker_RefusedHandshake(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()
	start := time.Now()
	if err := DialTicker(context.Background(), wsURL(t, srv), "k", "t", 2*time.Second); err == nil {
		t.Fatal("expected an error for a refused handshake")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("DialTicker did not honour its timeout")
	}
}

func TestDialTicker_Connects(t *testing.T) {
	got := make(chan url.Values, 1)
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.URL.Query()
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	if err := DialTicker(context.Background(), wsURL(t, srv), "kitekey", "acc-tok", 2*time.Second); err != nil {
		t.Fatal(err)
	}
	q := <-got
	if q.Get("api_key") != "kitekey" || q.Get("access_token") != "acc-tok" {
		t.Fatalf("query %v", q)
	}
}

type histCall struct {
	token    int
	iv       string
	from, to time.Time
}

type fakeHistorical struct {
	calls []histCall
	err   error
}

func (f *fakeHistorical) Historical(_ context.Context, token int, iv string, from, to time.Time, _, _ bool) ([]kiteconnect.HistoricalData, error) {
	f.calls = append(f.calls, histCall{token, iv, from, to})
	return nil, f.err
}

// The production probe asks for one week of NIFTY 50 day candles, on the app clock.
func TestNewCapabilityProbe_HistoricalRequest(t *testing.T) {
	now := time.Date(2026, 10, 12, 9, 30, 0, 0, clock.IST)
	h := &fakeHistorical{}
	p := NewCapabilityProbe(h, clock.NewFake(now), nil, slog.Default())
	if got := p.Probe(context.Background(), "k", "t"); got != PlanConnect {
		t.Fatalf("plan %s", got)
	}
	if len(h.calls) != 1 {
		t.Fatalf("historical calls %d", len(h.calls))
	}
	c := h.calls[0]
	if c.token != NiftyIndexToken || c.iv != "day" || !c.to.Equal(now) || !c.from.Equal(now.AddDate(0, 0, -7)) {
		t.Fatalf("call %+v", c)
	}
}

// Ticker URLs carry the credentials (raw and query-escaped); no DialTicker error may contain them.
func TestDialTicker_ErrorsNeverCarryCredentials(t *testing.T) {
	const key, tok = "api key+1", "tok/en+s3cret=="
	root := &url.URL{Scheme: "ws", Host: "[::1", Path: "/ws"} // url.Parse fails and echoes the full URL
	err := DialTicker(context.Background(), root, key, tok, time.Second)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, s := range []string{key, tok, url.QueryEscape(key), url.QueryEscape(tok)} {
		if strings.Contains(err.Error(), s) {
			t.Fatalf("error leaks %q: %s", s, err)
		}
	}
}

// A caller cancel ends DialTicker at once even while the handshake hangs.
func TestDialTicker_CtxCancelDuringHangingHandshake(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := ln.Accept() // never answer the handshake
			if err != nil {
				return
			}
			accepted <- c
		}
	}()
	defer func() {
		_ = ln.Close()
		for len(accepted) > 0 {
			_ = (<-accepted).Close()
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	res := make(chan error, 1)
	go func() {
		res <- DialTicker(ctx, &url.URL{Scheme: "ws", Host: ln.Addr().String(), Path: "/ws"}, "k", "t", 3*time.Second)
	}()
	c := <-accepted
	accepted <- c
	cancel()
	select {
	case err := <-res:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("DialTicker ignored the caller's cancel")
	}
}

// After a successful handshake the probe socket is closed again: the server sees EOF.
func TestDialTicker_ClosesSocketAfterConnect(t *testing.T) {
	closed := make(chan struct{})
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				close(closed)
				return
			}
		}
	}))
	defer srv.Close()
	if err := DialTicker(context.Background(), wsURL(t, srv), "k", "t", 2*time.Second); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("probe socket left open")
	}
}

// AcquireDial is one process-wide lock; a waiter gives up when its ctx ends; release is idempotent.
func TestAcquireDial(t *testing.T) {
	release, err := AcquireDial(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := AcquireDial(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second acquire while held: %v", err)
	}
	release()
	release()
	r2, err := AcquireDial(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r2()
}

// Longest-first: the key also occurs inside the token's escaped form. Replacing the key first would
// split that form and leave the rest of the token in the text.
func TestRedactCredentials_LongestFirst(t *testing.T) {
	const tok = "tok/en+s3cret"
	const key = "2Fen" // a substring of url.QueryEscape(tok) == "tok%2Fen%2Bs3cret"
	if !strings.Contains(url.QueryEscape(tok), key) {
		t.Fatal("test setup: key must occur inside the escaped token")
	}
	in := errors.New("dial wss://ws.kite.trade?access_token=" + url.QueryEscape(tok) + "&api_key=" + key + ": refused")
	got := RedactCredentials(in, key, tok).Error()
	for _, s := range []string{"s3cret", key, tok, url.QueryEscape(tok)} {
		if strings.Contains(got, s) {
			t.Fatalf("redacted text leaks %q: %s", s, got)
		}
	}
	if RedactCredentials(nil, key, tok) != nil {
		t.Fatal("nil error must stay nil")
	}
	plain := errors.New("no credentials here")
	if RedactCredentials(plain, key, tok) != plain {
		t.Fatal("an error without credentials must be returned unchanged")
	}
}

// A caller that already gave up gets PlanUnknown and no ticker attempt.
func TestProbe_CtxDoneReturnsUnknownWithoutTicker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &probeCalls{}
	p := newTestProbe(context.Canceled, nil, c)
	if got := p.Probe(ctx, "k", "t"); got != PlanUnknown || c.ticker != 0 {
		t.Fatalf("plan %s ticker calls %d", got, c.ticker)
	}
}
