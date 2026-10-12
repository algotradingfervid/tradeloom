// internal/kite/mock/zero_writes_test.go
package mock_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/feed"
	"tradeloom/internal/kite/mock"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/orders"
	"tradeloom/internal/secrets"
	"tradeloom/internal/service"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

const (
	key    = "kitekey1234abcd"
	secret = "s3cr3tvalue0987654321zyxwvutsrq"
	rtok1  = "rtok12345678"
	rtok2  = "rtok87654321"
	rtok3  = "rtok11223344"
)

// allowedRoutes is the complete set of (method, path) pairs the session/sync/service layer may send to Kite
// during a full cycle. Fail closed: anything else (any other method, any other path, a GET included) fails.
//
// It deliberately duplicates the default routes of mock.NewRecorder instead of being derived from them:
// the recorder's routes say what the fake can answer, this list says what the app may send. Deriving one
// from the other would let a route added to the recorder (or a test's rec.Route call) silently widen what
// the G-0 proof accepts. Keep the two in sync by hand; the "every allowed route exercised" check at the
// end of TestFullSessionCycle_SendsNoOrderWrites catches an entry here that the cycle no longer uses.
var allowedRoutes = map[string]bool{
	"POST /session/token":                    true, // login exchange
	"DELETE /session/token":                  true, // logout
	"GET /user/profile":                      true, // restore at startup, Profile
	"GET /instruments/historical/256265/day": true, // capability probe
	"GET /orders":                            true,
	"GET /portfolio/positions":               true,
	"GET /portfolio/holdings":                true,
	"GET /quote/ltp":                         true, // Personal-plan LTP fallback
}

// leakSource reports requests that bypassed the recorder, as finished violation messages.
type leakSource interface{ Leaks() []string }

// violations lists every request that breaks G-0: a write (anything but GET, except POST/DELETE
// /session/token), a request no recorder route answered, a route outside allowedRoutes, the API secret
// anywhere in a request (form, headers, query string, raw body, path), and every request a leak source saw
// (a host other than the recorder, anything sent through http.DefaultTransport).
func violations(rec *mock.Recorder, leaks ...leakSource) []string {
	var out []string
	unmatched := map[string]bool{}
	for _, r := range rec.Unmatched() {
		unmatched[r.Method+" "+r.Path] = true
	}
	for _, r := range rec.Requests() {
		id := r.Method + " " + r.Path
		if mock.IsWrite(r) {
			out = append(out, "write request sent to Kite: "+id)
		}
		if unmatched[id] {
			out = append(out, "request to an unmocked route: "+id)
		}
		if !allowedRoutes[id] {
			out = append(out, "request outside the allowed route set: "+id)
		}
		if r.BodyErr != nil {
			out = append(out, "request body not read in full, secret scan incomplete: "+id)
		}
		out = append(out, secretLeaks(r, id)...)
	}
	for _, l := range leaks {
		out = append(out, l.Leaks()...)
	}
	return out
}

// secretLeaks reports the API secret in any part of r, raw or percent-decoded, in a fixed order.
func secretLeaks(r mock.Request, id string) []string {
	has := func(v string) bool {
		if strings.Contains(v, secret) {
			return true
		}
		u, err := url.QueryUnescape(v)
		return err == nil && strings.Contains(u, secret)
	}
	var out []string
	for k, vs := range r.Form {
		if has(k) || has(strings.Join(vs, "\x00")) {
			out = append(out, "API secret sent to Kite in a form value: "+id)
			break
		}
	}
	names := make([]string, 0, len(r.Header))
	for k := range r.Header {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if has(k) || has(strings.Join(r.Header[k], "\x00")) {
			out = append(out, "API secret sent to Kite in header "+k+": "+id)
		}
	}
	if has(r.RawQuery) {
		out = append(out, "API secret sent to Kite in the query string: "+id)
	}
	if has(string(r.Body)) {
		out = append(out, "API secret sent to Kite in the request body: "+id)
	}
	if has(r.Path) {
		out = append(out, "API secret sent to Kite in the path: "+id)
	}
	return out
}

func count(rec *mock.Recorder, method, path string) int {
	n := 0
	for _, r := range rec.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

// hostGuard sits outside the NFR-04 allow-list transport and records every request whose host is not
// the recorder's, so a request that bypassed the recorder cannot go unnoticed.
type hostGuard struct {
	next http.RoundTripper
	host string
	mu   sync.Mutex
	bad  []string
}

func (g *hostGuard) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != g.host {
		g.mu.Lock()
		g.bad = append(g.bad, req.Method+" "+req.URL.Scheme+"://"+req.URL.Host+req.URL.Path)
		g.mu.Unlock()
	}
	return g.next.RoundTrip(req)
}

func (g *hostGuard) Leaks() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]string, 0, len(g.bad))
	for _, b := range g.bad {
		out = append(out, "request to a host other than the recorder: "+b)
	}
	return out
}

// refusingTransport replaces http.DefaultTransport during the full cycle: every request that reaches it
// (http.Get, http.DefaultClient, a client or transport built with a nil RoundTripper) bypassed the
// allow-list transport and the recorder. It is refused and recorded.
type refusingTransport struct {
	mu   sync.Mutex
	seen []string
}

var errDefaultTransport = errors.New("zero-writes test: http.DefaultTransport is refused")

func (d *refusingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
	d.mu.Lock()
	d.seen = append(d.seen, req.Method+" "+req.URL.String())
	d.mu.Unlock()
	return nil, errDefaultTransport
}

func (d *refusingTransport) Leaks() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, 0, len(d.seen))
	for _, s := range d.seen {
		out = append(out, "request through http.DefaultTransport (refused): "+s)
	}
	return out
}

// refuseDefaultTransport swaps http.DefaultTransport for a refusingTransport until the test ends. Tests
// in this package do not run in parallel, so the global swap is not observed by another test.
func refuseDefaultTransport(t *testing.T) *refusingTransport {
	t.Helper()
	orig := http.DefaultTransport
	d := &refusingTransport{}
	http.DefaultTransport = d
	t.Cleanup(func() { http.DefaultTransport = orig })
	return d
}

type memSecrets struct {
	mu sync.Mutex
	m  map[secrets.Key]string
}

func (s *memSecrets) Get(k secrets.Key) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.m[k]; ok {
		return v, nil
	}
	return "", secrets.ErrNotFound
}
func (s *memSecrets) Set(k secrets.Key, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}
func (s *memSecrets) Delete(k secrets.Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
	return nil
}
func (s *memSecrets) Backend() string { return "memory" }

// countFeed stands in for *feed.Feed (no WebSocket in this test: the ticker is not an HTTP write path).
type countFeed struct {
	mu            sync.Mutex
	starts, stops int
}

func (f *countFeed) Start(context.Context, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	return nil
}
func (f *countFeed) Stop() { f.mu.Lock(); f.stops++; f.mu.Unlock() }
func (f *countFeed) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts, f.stops
}

type nopLive struct{}

func (nopLive) Subscribe(ticker.Consumer, string, []uint32, ticker.Mode) error { return nil }
func (nopLive) Unsubscribe(ticker.Consumer, string, []uint32)                  {}
func (nopLive) Release(ticker.Consumer, string)                                {}

// stack is one app run wired like cmd/tradeloom (Task 23): the real P0-2 kite.Client and limiter on the
// allow-list transport, the real kite.Session, orders.Sync and service.Service.
type stack struct {
	sess   *kite.Session
	svc    *service.Service
	syncer *orders.Sync
	feed   *countFeed
	stop   func()
}

func newStack(t *testing.T, rec *mock.Recorder, guard http.RoundTripper, db *store.DB, sec secrets.Store, clk *clock.Fake) *stack {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	b := bus.New(nil)
	st, err := settings.Load(ctx, db.SettingsRepo(), b)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	lim := ratelimit.New(clk, ratelimit.Options{Bus: b, Log: log})
	lim.Run(ctx)
	relay := &kite.SessionErrorRelay{}
	client := kite.New(kite.Options{BaseURI: rec.URL(), Limiter: lim, Clock: clk, OnSessionError: relay.Handle,
		Transport: guard, Jitter: func() time.Duration { return 0 }, Log: log})
	fd := &countFeed{}
	sess := kite.NewSession(kite.SessionDeps{
		Settings: st, Secrets: sec, Repo: store.NewSessionsRepo(db), Clock: clk, Bus: b,
		Auth: kite.NewAuth(&http.Client{Timeout: 10 * time.Second, Transport: guard}, rec.URL()), Profiles: client, Tokens: client,
		Limiter: lim, Feed: fd, Probe: kite.NewCapabilityProbe(client, clk, nil, log), Log: log,
	})
	relay.Set(sess.HandleTokenError)
	syncer := orders.NewSync(orders.SyncDeps{Read: client, Cache: store.NewOrdersCacheRepo(db), Bus: b, Clock: clk, Live: nopLive{},
		Log: log, LiveNotRunning: feed.ErrNotRunning})
	svc := service.New(service.Deps{Session: sess, Orders: syncer, Settings: st, Watchlists: store.NewWatchlistsRepo(db),
		Live: nopLive{}, Clock: clk, Bus: b, Log: log})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); svc.RunSession(ctx) }()
	go func() { defer wg.Done(); svc.Run(ctx) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			wg.Wait()
			b.Close()
		})
	}
	t.Cleanup(stop) // registered after storetest.Open: runs before the database closes
	return &stack{sess: sess, svc: svc, syncer: syncer, feed: fd, stop: stop}
}

// eventually waits (real time) for cond.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// advanceUntil moves the fake clock in steps until cond holds.
func advanceUntil(t *testing.T, clk *clock.Fake, step time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out (clock %s) waiting for %s", clk.Now().Format(time.RFC3339), what)
		}
		clk.Advance(step)
		time.Sleep(3 * time.Millisecond)
	}
}

// settleThenAssertNoTraffic advances the clock by d and asserts that Kite saw no request meanwhile
// (Global Constraint: no Kite calls between the end of a session and the next login).
func settleThenAssertNoTraffic(t *testing.T, rec *mock.Recorder, clk *clock.Fake, d time.Duration, when string) {
	t.Helper()
	time.Sleep(50 * time.Millisecond) // let in-flight calls of the ended session finish
	before := len(rec.Requests())
	for end := clk.Now().Add(d); clk.Now().Before(end); {
		clk.Advance(5 * time.Second)
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	if after := rec.Requests(); len(after) != before {
		for _, r := range after[before:] {
			t.Errorf("Kite request %s, after %s: %s %s", clk.Now().Format(time.RFC3339), when, r.Method, r.Path)
		}
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func loginState(t *testing.T, loginURL string) string {
	t.Helper()
	u, err := url.Parse(loginURL)
	if err != nil {
		t.Fatal(err)
	}
	rp, err := url.ParseQuery(u.Query().Get("redirect_params"))
	if err != nil || rp.Get("state") == "" {
		t.Fatalf("login URL carries no state: %s", loginURL)
	}
	return rp.Get("state")
}

// TestFullSessionCycle_SendsNoOrderWrites drives two days of realistic use through the real session,
// sync and service layers against a recording Kite and proves that the only non-GET requests are the
// login exchange and the logout (ARCH §2.5 check 3, G-0):
//
//	day 1: startup restore (nothing stored) → credentials → login by paste → Connect-plan probe → risk ack
//	       → profile → initial sync → 30 s and 5 min polls → manual refresh → 06:00 IST expiry → silence;
//	day 2: login by the loopback callback → Personal-plan probe → LTP polling → app restart (token restored
//	       through /user/profile) → logout → silence;
//	day 3: login by paste in market hours → sync running → Kite revokes the token (TokenException on the
//	       order poll) → session ends mid-session → silence.
//
// http.DefaultTransport is replaced by a refusing recorder for the whole cycle, so a request sent by any
// path that bypasses the allow-list transport is refused and fails the test.
func TestFullSessionCycle_SendsNoOrderWrites(t *testing.T) {
	rec := mock.NewRecorder(t)
	dflt := refuseDefaultTransport(t)
	db := storetest.Open(t)
	recURL, err := url.Parse(rec.URL())
	if err != nil {
		t.Fatal(err)
	}
	guard := &hostGuard{next: kite.NewAllowListTransport(rec.Client().Transport, []string{recURL.Hostname()}), host: recURL.Host}
	sec := &memSecrets{m: map[secrets.Key]string{}}
	clk := clock.NewFake(time.Date(2026, 10, 12, 9, 2, 0, 0, clock.IST))
	ctx := context.Background()
	defer func() {
		for _, v := range violations(rec, guard, dflt) {
			t.Error(v)
		}
	}()

	// ---- day 1 ----
	a := newStack(t, rec, guard, db, sec, clk)
	if st := a.svc.Restore(ctx); st.State != string(kite.StateNoCredentials) {
		t.Fatalf("restore with nothing stored: %+v", st)
	}
	port := freePort(t)
	if _, err := a.svc.SetRedirectPort(ctx, port); err != nil {
		t.Fatal(err)
	}
	if _, err := a.svc.SaveCredentials(ctx, service.CredentialsInput{APIKey: key, APISecret: secret}); err != nil {
		t.Fatal(err)
	}
	ls, err := a.svc.BeginLogin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Paste fallback: the user copies the redirect address from the browser.
	pasted := ls.RedirectURL + "?request_token=" + rtok1 + "&action=login&status=success&state=" + loginState(t, ls.LoginURL)
	if st, err := a.svc.CompleteLogin(ctx, pasted); err != nil || st.State != string(kite.StateReady) {
		t.Fatalf("paste login: %+v, %v", st, err)
	}
	eventually(t, "Connect plan from the probe", func() bool { return a.svc.SessionStatus(ctx).DataPlan == string(kite.PlanConnect) })
	if _, err := a.svc.AckRiskDisclosure(ctx); err != nil {
		t.Fatal(err)
	}
	if p, err := a.svc.Profile(ctx); err != nil || p.UserID == "" {
		t.Fatalf("profile: %+v, %v", p, err)
	}
	eventually(t, "initial portfolio sync", func() bool {
		return count(rec, http.MethodGet, "/orders") >= 1 && count(rec, http.MethodGet, "/portfolio/holdings") >= 1
	})
	advanceUntil(t, clk, 5*time.Second, "30 s order/position poll", func() bool {
		return count(rec, http.MethodGet, "/orders") >= 2 && count(rec, http.MethodGet, "/portfolio/positions") >= 2
	})
	advanceUntil(t, clk, 30*time.Second, "5 min holdings poll", func() bool { return count(rec, http.MethodGet, "/portfolio/holdings") >= 2 })
	// Manual refresh (the façade's RefreshPortfolio is Task 20; it calls this Sync.Refresh).
	n := count(rec, http.MethodGet, "/orders")
	if err := a.syncer.Refresh(ctx, orders.ScopeAll); err != nil {
		t.Fatal(err)
	}
	if count(rec, http.MethodGet, "/orders") <= n {
		t.Fatal("manual refresh sent no GET /orders")
	}
	if h := a.syncer.Holdings(); !h.Summary.Empty {
		t.Fatalf("holdings %+v", h)
	}
	// 06:00 IST the next morning, with the sync and the (fake) ticker running.
	clk.Set(time.Date(2026, 10, 13, 5, 59, 50, 0, clock.IST))
	advanceUntil(t, clk, time.Second, "06:00 expiry", func() bool { return a.svc.SessionStatus(ctx).State == string(kite.StateExpired) })
	if st := a.svc.SessionStatus(ctx); st.ErrorCode != kite.CodeSessionExpired {
		t.Fatalf("expired status %+v", st)
	}
	if starts, stops := a.feed.counts(); starts != 1 || stops < 1 {
		t.Fatalf("feed starts=%d stops=%d", starts, stops)
	}
	if _, err := sec.Get(secrets.KeyAccessToken); err != secrets.ErrNotFound {
		t.Fatalf("token not deleted at expiry: %v", err)
	}
	settleThenAssertNoTraffic(t, rec, clk, 10*time.Minute, "the 06:00 expiry")

	// ---- day 2: a Personal-plan key, login through the loopback callback ----
	rec.Route(http.MethodGet, "/instruments/historical/256265/day", http.StatusForbidden,
		`{"status":"error","error_type":"PermissionException","message":"Insufficient permission for that call."}`)
	rec.Route(http.MethodPost, "/session/token", http.StatusOK,
		`{"status":"success","data":{"user_id":"AB1234","user_name":"Asha Bhat","api_key":"k","access_token":"acc-mock-0002"}}`)
	rec.Route(http.MethodGet, "/portfolio/holdings", http.StatusOK,
		`{"status":"success","data":[{"tradingsymbol":"INFY","exchange":"NSE","instrument_token":408065,"isin":"INE009A01021","product":"CNC","quantity":10,"average_price":1500,"last_price":1600,"close_price":1590,"pnl":1000}]}`)
	rec.Route(http.MethodGet, "/quote/ltp", http.StatusOK, `{"status":"success","data":{"NSE:INFY":{"instrument_token":408065,"last_price":1610}}}`)
	ls, err = a.svc.BeginLogin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The browser's GET to the loopback callback goes through its own transport: http.DefaultTransport refuses.
	browser := &http.Client{Transport: &http.Transport{}}
	defer browser.CloseIdleConnections()
	resp, err := browser.Get(ls.RedirectURL + "?request_token=" + rtok2 + "&action=login&status=success&state=" + loginState(t, ls.LoginURL))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("callback status %d", resp.StatusCode)
	}
	eventually(t, "Personal plan from the probe", func() bool { return a.svc.SessionStatus(ctx).DataPlan == string(kite.PlanNone) })
	eventually(t, "holdings loaded", func() bool { return !a.syncer.Holdings().Summary.Empty })
	advanceUntil(t, clk, 10*time.Second, "Personal-plan LTP poll", func() bool { return count(rec, http.MethodGet, "/quote/ltp") >= 1 })

	// ---- app restart: the stored token is resumed (GET /user/profile), then the user logs out ----
	a.stop()
	b := newStack(t, rec, guard, db, sec, clk)
	profiles := count(rec, http.MethodGet, "/user/profile")
	if st := b.svc.Restore(ctx); st.State != string(kite.StateReady) {
		t.Fatalf("restore after restart: %+v", st)
	}
	if count(rec, http.MethodGet, "/user/profile") <= profiles {
		t.Fatal("restore did not check the token with GET /user/profile")
	}
	eventually(t, "sync after restore", func() bool { return !b.syncer.Holdings().Summary.Empty })
	if res, err := b.svc.Logout(ctx); err != nil || !res.RemoteInvalidated {
		t.Fatalf("logout: %+v, %v", res, err)
	}
	if st := b.svc.SessionStatus(ctx); st.State != string(kite.StateLoggedOut) {
		t.Fatalf("after logout %+v", st)
	}
	settleThenAssertNoTraffic(t, rec, clk, 10*time.Minute, "the logout")

	// ---- day 3: Kite revokes the token mid-session (TokenException on a read) ----
	clk.Set(time.Date(2026, 10, 13, 9, 20, 0, 0, clock.IST)) // market hours: the 30 s order poll runs
	rec.Route(http.MethodPost, "/session/token", http.StatusOK,
		`{"status":"success","data":{"user_id":"AB1234","user_name":"Asha Bhat","api_key":"k","access_token":"acc-mock-0003"}}`)
	ls, err = b.svc.BeginLogin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ordersBefore := count(rec, http.MethodGet, "/orders")
	pasted = ls.RedirectURL + "?request_token=" + rtok3 + "&action=login&status=success&state=" + loginState(t, ls.LoginURL)
	if st, err := b.svc.CompleteLogin(ctx, pasted); err != nil || st.State != string(kite.StateReady) {
		t.Fatalf("day-3 paste login: %+v, %v", st, err)
	}
	eventually(t, "day-3 initial sync", func() bool { return count(rec, http.MethodGet, "/orders") > ordersBefore })
	_, stopsBefore := b.feed.counts()
	rec.Route(http.MethodGet, "/orders", http.StatusForbidden,
		`{"status":"error","error_type":"TokenException","message":"Incorrect api_key or access_token."}`)
	advanceUntil(t, clk, 5*time.Second, "TokenException ends the session", func() bool {
		return b.svc.SessionStatus(ctx).State == string(kite.StateExpired)
	})
	if st := b.svc.SessionStatus(ctx); st.ErrorCode != kite.CodeTokenRejected {
		t.Fatalf("after TokenException %+v", st)
	}
	if _, stops := b.feed.counts(); stops <= stopsBefore {
		t.Fatalf("feed not stopped after TokenException (stops %d → %d)", stopsBefore, stops)
	}
	if _, err := sec.Get(secrets.KeyAccessToken); err != secrets.ErrNotFound {
		t.Fatalf("token not deleted after TokenException: %v", err)
	}
	settleThenAssertNoTraffic(t, rec, clk, 10*time.Minute, "the TokenException")

	// ---- what Kite saw ----
	var logins, logouts int
	for _, r := range rec.Requests() {
		switch {
		case r.Method == http.MethodPost && r.Path == "/session/token":
			want := map[string]string{
				rtok1: fmt.Sprintf("%x", sha256.Sum256([]byte(key+rtok1+secret))),
				rtok2: fmt.Sprintf("%x", sha256.Sum256([]byte(key+rtok2+secret))),
				rtok3: fmt.Sprintf("%x", sha256.Sum256([]byte(key+rtok3+secret))),
			}[r.Form.Get("request_token")]
			if want == "" || r.Form.Get("checksum") != want {
				t.Errorf("login exchange with request_token %q and a wrong checksum", r.Form.Get("request_token"))
			}
			logins++
		case r.Method == http.MethodDelete && r.Path == "/session/token":
			if r.Form.Get("access_token") != "acc-mock-0002" {
				t.Errorf("logout invalidated %q, want the day-2 token", r.Form.Get("access_token"))
			}
			logouts++
		}
	}
	if logins != 3 || logouts != 1 {
		t.Errorf("logins=%d logouts=%d, want 3 and 1", logins, logouts)
	}
	for id := range allowedRoutes {
		m, p, _ := strings.Cut(id, " ")
		if count(rec, m, p) == 0 {
			t.Errorf("the cycle never exercised %s", id)
		}
	}
}

// TestRecorder_DetectsEveryWrite is the RED proof: each order-write route of Kite (orders, GTT, alerts,
// mutual funds, position conversion, holdings authorisation), any other method on the session endpoint and
// any unknown path, sent through the mock, is reported by the same check the full cycle uses, and the
// report names the entry that tripped: the write, the unmocked route and the route outside the allow-list.
func TestRecorder_DetectsEveryWrite(t *testing.T) {
	cases := []struct {
		method, path string
		write        bool // the "write request sent to Kite" entry is expected
	}{
		{"POST", "/orders/regular", true}, {"POST", "/orders/amo", true}, {"POST", "/orders/co", true}, {"POST", "/orders/iceberg", true}, {"POST", "/orders/auction", true},
		{"PUT", "/orders/regular/1", true}, {"DELETE", "/orders/regular/1", true}, {"DELETE", "/orders/co/1", true},
		{"PUT", "/portfolio/positions", true}, {"POST", "/portfolio/holdings/authorise", true},
		{"POST", "/gtt/triggers", true}, {"PUT", "/gtt/triggers/5", true}, {"DELETE", "/gtt/triggers/5", true},
		{"POST", "/alerts", true}, {"PUT", "/alerts/0f3a", true}, {"DELETE", "/alerts", true},
		{"POST", "/mf/orders", true}, {"DELETE", "/mf/orders/1", true}, {"POST", "/mf/sips", true}, {"PUT", "/mf/sips/1", true}, {"DELETE", "/mf/sips/1", true},
		{"PUT", "/session/token", true}, {"PATCH", "/session/token", true}, {"POST", "/session/token/", true}, {"POST", "/session/token/x", true},
		{"PATCH", "/orders", true}, {"OPTIONS", "/orders", true}, {"HEAD", "/orders", true},
		{"GET", "/gtt/triggers", false}, {"GET", "/no/such/route", false},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			rec := mock.NewRecorder(t)
			req, err := http.NewRequest(c.method, rec.URL()+c.path, strings.NewReader(url.Values{"variety": {"regular"}, "quantity": {"1"}}.Encode()))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := rec.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			id := c.method + " " + c.path
			var want []string
			if c.write {
				want = append(want, "write request sent to Kite: "+id)
			}
			want = append(want, "request to an unmocked route: "+id, "request outside the allowed route set: "+id)
			assertViolations(t, violations(rec), want)
		})
	}
	// A deliberate order write that the test itself routes (so the recorder answers it) is still a write.
	t.Run("routed order POST", func(t *testing.T) {
		rec := mock.NewRecorder(t)
		rec.Route(http.MethodPost, "/orders/regular", http.StatusOK, `{"status":"success","data":{"order_id":"1"}}`)
		resp, err := rec.Client().PostForm(rec.URL()+"/orders/regular", url.Values{"tradingsymbol": {"INFY"}, "transaction_type": {"BUY"}})
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		assertViolations(t, violations(rec), []string{
			"write request sent to Kite: POST /orders/regular", "request outside the allowed route set: POST /orders/regular"})
	})
	// The API secret on an allowed route, in each part of the request the recorder keeps.
	secretCases := []struct {
		name  string
		build func(base string) (*http.Request, error)
		want  []string
	}{
		{"header", func(base string) (*http.Request, error) {
			r, err := http.NewRequest(http.MethodGet, base+"/orders", nil)
			if err == nil {
				r.Header.Set("X-Api-Secret", secret)
			}
			return r, err
		}, []string{"API secret sent to Kite in header X-Api-Secret: GET /orders"}},
		{"query", func(base string) (*http.Request, error) {
			return http.NewRequest(http.MethodGet, base+"/orders?api_secret="+secret, nil)
		}, []string{"API secret sent to Kite in a form value: GET /orders", "API secret sent to Kite in the query string: GET /orders"}},
		// A DELETE body is not parsed into Form: only the raw-body scan sees it.
		{"raw body", func(base string) (*http.Request, error) {
			r, err := http.NewRequest(http.MethodDelete, base+"/session/token", strings.NewReader("api_key=k&api_secret="+secret))
			if err == nil {
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			return r, err
		}, []string{"API secret sent to Kite in the request body: DELETE /session/token"}},
	}
	for _, c := range secretCases {
		t.Run("secret in "+c.name, func(t *testing.T) {
			rec := mock.NewRecorder(t)
			req, err := c.build(rec.URL())
			if err != nil {
				t.Fatal(err)
			}
			resp, err := rec.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			assertViolations(t, violations(rec), c.want)
		})
	}
	// A request that left the recorder's host is reported too.
	t.Run("foreign host", func(t *testing.T) {
		g := &hostGuard{next: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, kite.ErrHostNotAllowed }), host: "127.0.0.1:1"}
		if _, err := (&http.Client{Transport: g}).Get("https://api.kite.trade/orders"); err == nil {
			t.Fatal("foreign request succeeded")
		}
		assertViolations(t, violations(mock.NewRecorder(t), g), []string{"request to a host other than the recorder: GET https://api.kite.trade/orders"})
	})
	// An order POST through http.DefaultClient (any path that bypasses the allow-list transport) is refused and reported.
	t.Run("default transport", func(t *testing.T) {
		d := refuseDefaultTransport(t)
		// kite.invalid (RFC 6761) never resolves: if the swap failed, the request could not reach Kite.
		if _, err := http.PostForm("https://kite.invalid/orders/regular", url.Values{"quantity": {"1"}}); !errors.Is(err, errDefaultTransport) {
			t.Fatalf("POST through http.DefaultClient: %v", err)
		}
		assertViolations(t, violations(mock.NewRecorder(t), d), []string{"request through http.DefaultTransport (refused): POST https://kite.invalid/orders/regular"})
	})
	// A body cut short (here: Content-Length larger than what arrives) is recorded with its read error and
	// reported, because the secret scan only saw part of it.
	t.Run("unreadable body", func(t *testing.T) {
		rec := mock.NewRecorder(t)
		u, err := url.Parse(rec.URL())
		if err != nil {
			t.Fatal(err)
		}
		conn, err := net.Dial("tcp", u.Host)
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.WriteString(conn, "POST /session/token HTTP/1.1\r\nHost: "+u.Host+"\r\n"+
			"Content-Type: application/x-www-form-urlencoded\r\nContent-Length: 100\r\n\r\napi_key=k")
		_ = conn.Close()
		if err != nil {
			t.Fatal(err)
		}
		eventually(t, "the truncated request is recorded", func() bool { return len(rec.Requests()) == 1 })
		if r := rec.Requests()[0]; r.BodyErr == nil || string(r.Body) != "api_key=k" {
			t.Fatalf("body %q, err %v", r.Body, r.BodyErr)
		}
		assertViolations(t, violations(rec), []string{"request body not read in full, secret scan incomplete: POST /session/token"})
	})
	// And a clean recorder reports nothing.
	assertViolations(t, violations(mock.NewRecorder(t)), nil)
}

// assertViolations compares the exact violation list, so a RED case proves which entry tripped.
func assertViolations(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("violations\n got %q\nwant %q", got, want)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIsWrite(t *testing.T) {
	writes := []mock.Request{{Method: "POST", Path: "/orders/regular"}, {Method: "PUT", Path: "/orders/regular/1"}, {Method: "DELETE", Path: "/orders/regular/1"},
		{Method: "PUT", Path: "/portfolio/positions"}, {Method: "POST", Path: "/gtt/triggers"}, {Method: "DELETE", Path: "/gtt/triggers/5"}, {Method: "POST", Path: "/alerts"},
		{Method: "PUT", Path: "/alerts/x"}, {Method: "DELETE", Path: "/alerts"}, {Method: "POST", Path: "/mf/orders"}, {Method: "POST", Path: "/mf/sips"},
		{Method: "POST", Path: "/portfolio/holdings/authorise"}, {Method: "PUT", Path: "/session/token"}, {Method: "PATCH", Path: "/session/token"},
		{Method: "POST", Path: "/session/token/"}, {Method: "post", Path: "/session/token"}, {Method: "HEAD", Path: "/orders"}, {Method: "", Path: "/orders"}}
	reads := []mock.Request{{Method: "GET", Path: "/orders"}, {Method: "POST", Path: "/session/token"}, {Method: "DELETE", Path: "/session/token"}, {Method: "GET", Path: "/portfolio/holdings"}}
	for _, r := range writes {
		if !mock.IsWrite(r) {
			t.Errorf("%s %s not flagged", r.Method, r.Path)
		}
	}
	for _, r := range reads {
		if mock.IsWrite(r) {
			t.Errorf("%s %s flagged", r.Method, r.Path)
		}
	}
}
