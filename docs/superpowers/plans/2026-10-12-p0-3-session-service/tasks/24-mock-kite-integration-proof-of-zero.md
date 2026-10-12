> **P0-3 Session & Service Layer Implementation Plan — Task 24.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 24: Mock-Kite integration — proof of zero order-write calls (G-0)

**Files:**
- Create: `internal/kite/mock/recorder.go`
- Test: `internal/kite/mock/zero_writes_test.go`

**Interfaces:**
- Consumes: Tasks 6–11 (real `kite.Session`), Task 19 (real `orders.Sync`), Task 14 (real `service.Service`), Task 5 `storetest.Open`; P0-1 `bus.New(nil)`, `settings.Load`, `settings.Set(ctx, …)`. gokiteconnect read calls are used directly in the test adapter (verified signatures): `GetUserProfile() (UserProfile, error)`, `GetHistoricalData(int, string, time.Time, time.Time, bool, bool) ([]HistoricalData, error)`, `GetOrders() (Orders, error)`, `GetPositions() (Positions, error)`, `GetHoldings() (Holdings, error)`, `GetLTP(...string) (QuoteLTP, error)`. The file joins P0-2's `package mock` (`server.go`, `fake.go`, `packets.go`); the names below (`Request`, `Recorder`, `NewRecorder`, `IsWrite`, unexported `route`) collide with none of P0-2's.
- Produces:
  ```go
  type Request struct{ Method, Path string; Form url.Values }
  type Recorder struct{ /* httptest server */ }
  func NewRecorder(t testing.TB) *Recorder // default routes for session, profile, historical, orders, portfolio, LTP
  func (r *Recorder) Route(method, path string, status int, body string)
  func (r *Recorder) URL() string
  func (r *Recorder) Client() *http.Client
  func (r *Recorder) Requests() []Request
  func IsWrite(req Request) bool // any non-GET request except POST/DELETE /session/token
  ```

The test lives in `internal/kite/mock` because only `internal/kite/...` may import gokiteconnect. It uses its own recorder instead of P0-2's `mock.Server` fixtures, so it depends on no fixture names. A full login → probe → sync → refresh → logout cycle must send zero write requests. This is ARCH §2.5 check 3. Check 2 (`go tool nm`) is Step 5.

- [ ] **Step 1: Write the failing test**

```go
// internal/kite/mock/zero_writes_test.go
package mock_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/mock"
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
	rtok   = "rtok12345678"
)

// kcRead adapts *kiteconnect.Client directly to the Session's and Sync's read interfaces, so this test
// proves the request set of the session/sync layer alone (P0-2's kite.Client and its limiter are proven
// against the mock server in P0-2 Task 9 and never add non-GET routes).
type kcRead struct {
	mu   sync.Mutex
	c    *kiteconnect.Client
	hc   *http.Client
	base string
}

func (k *kcRead) client() *kiteconnect.Client { k.mu.Lock(); defer k.mu.Unlock(); return k.c }
func (k *kcRead) SetAPIKey(apiKey string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	c := kiteconnect.New(apiKey)
	c.SetHTTPClient(k.hc)
	c.SetBaseURI(k.base)
	k.c = c
}
func (k *kcRead) SetAccessToken(t string) { k.client().SetAccessToken(t) }
func (k *kcRead) Profile(context.Context) (kiteconnect.UserProfile, error) { return k.client().GetUserProfile() }
func (k *kcRead) Historical(_ context.Context, tok int, iv string, from, to time.Time, cont, oi bool) ([]kiteconnect.HistoricalData, error) {
	return k.client().GetHistoricalData(tok, iv, from, to, cont, oi)
}
func (k *kcRead) Orders(context.Context) (kite.Orders, error)       { return k.client().GetOrders() }
func (k *kcRead) Positions(context.Context) (kite.Positions, error) { return k.client().GetPositions() }
func (k *kcRead) Holdings(context.Context) (kite.Holdings, error)   { return k.client().GetHoldings() }
func (k *kcRead) LTP(_ context.Context, ids ...string) (kite.QuoteLTP, error) { return k.client().GetLTP(ids...) }

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
func (s *memSecrets) Set(k secrets.Key, v string) error { s.mu.Lock(); defer s.mu.Unlock(); s.m[k] = v; return nil }
func (s *memSecrets) Delete(k secrets.Key) error         { s.mu.Lock(); defer s.mu.Unlock(); delete(s.m, k); return nil }
func (s *memSecrets) Backend() string                    { return "memory" }

type nopFeed struct{}

func (nopFeed) Start(context.Context, string, string) error { return nil }
func (nopFeed) Stop()                                       {}

type nopLimiter struct{}

func (nopLimiter) PauseAll(string) {}
func (nopLimiter) Resume()         {}

type nopLive struct{}

func (nopLive) Subscribe(ticker.Consumer, string, []uint32, ticker.Mode) error { return nil }
func (nopLive) Unsubscribe(ticker.Consumer, string, []uint32)                  {}
func (nopLive) Release(ticker.Consumer, string)                                {}

func TestFullSessionCycle_SendsNoOrderWrites(t *testing.T) {
	rec := mock.NewRecorder(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := storetest.Open(t)
	b := bus.New(nil)
	t.Cleanup(b.Close)
	st, err := settings.Load(ctx, db.SettingsRepo(), b)
	if err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(time.Date(2026, 10, 12, 9, 2, 0, 0, clock.IST))
	rd := &kcRead{hc: rec.Client(), base: rec.URL()}
	rd.SetAPIKey(key)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := kite.NewSession(kite.SessionDeps{
		Settings: st, Secrets: &memSecrets{m: map[secrets.Key]string{}}, Repo: store.NewSessionsRepo(db), Clock: clk, Bus: b,
		Auth: kite.NewAuth(rec.Client(), rec.URL()), Profiles: rd, Tokens: rd, Limiter: nopLimiter{}, Feed: nopFeed{},
		Probe: kite.NewCapabilityProbe(rd, clk, nil, log), Log: log,
	})
	syncer := orders.NewSync(orders.SyncDeps{Read: rd, Cache: store.NewOrdersCacheRepo(db), Bus: b, Clock: clk, Live: nopLive{}, Log: log})
	svc := service.New(service.Deps{Session: sess, Orders: syncer, Settings: st, Watchlists: store.NewWatchlistsRepo(db), Live: nopLive{}, Clock: clk, Bus: b, Log: log})
	go svc.Run(ctx)

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	if err := settings.Set(ctx, st, kite.SettingRedirectPort, port); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveCredentials(ctx, service.CredentialsInput{APIKey: key, APISecret: secret}); err != nil {
		t.Fatal(err)
	}
	ls, err := svc.BeginLogin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(ls.LoginURL)
	rp, _ := url.ParseQuery(u.Query().Get("redirect_params"))
	resp, err := http.Get(ls.RedirectURL + "?request_token=" + rtok + "&action=login&status=success&state=" + rp.Get("state"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	deadline := time.Now().Add(3 * time.Second)
	for svc.SessionStatus(ctx).DataPlan != "connect" {
		if time.Now().After(deadline) {
			t.Fatalf("status %+v", svc.SessionStatus(ctx))
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := svc.RefreshPortfolio(ctx, "all"); err != nil {
		t.Fatal(err)
	}
	if h := svc.Holdings(ctx); !h.Summary.Empty {
		t.Fatalf("holdings %+v", h)
	}
	if _, err := svc.Logout(ctx); err != nil {
		t.Fatal(err)
	}

	var sawLogin, sawLogout, sawOrders, sawHistorical bool
	for _, r := range rec.Requests() {
		if mock.IsWrite(r) {
			t.Errorf("write request sent to Kite: %s %s", r.Method, r.Path)
		}
		for _, vs := range r.Form {
			for _, v := range vs {
				if v == secret {
					t.Errorf("API secret sent to Kite in %s %s", r.Method, r.Path)
				}
			}
		}
		switch {
		case r.Method == http.MethodPost && r.Path == "/session/token":
			sawLogin = r.Form.Get("checksum") == fmt.Sprintf("%x", sha256.Sum256([]byte(key+rtok+secret)))
		case r.Method == http.MethodDelete && r.Path == "/session/token":
			sawLogout = true
		case r.Method == http.MethodGet && r.Path == "/orders":
			sawOrders = true
		case r.Method == http.MethodGet && r.Path == "/instruments/historical/256265/day":
			sawHistorical = true
		}
	}
	if !sawLogin || !sawLogout || !sawOrders || !sawHistorical {
		t.Fatalf("login=%v logout=%v orders=%v historical=%v", sawLogin, sawLogout, sawOrders, sawHistorical)
	}
}

func TestIsWrite(t *testing.T) {
	writes := []mock.Request{{Method: "POST", Path: "/orders/regular"}, {Method: "PUT", Path: "/orders/regular/1"}, {Method: "DELETE", Path: "/orders/regular/1"},
		{Method: "PUT", Path: "/portfolio/positions"}, {Method: "POST", Path: "/gtt/triggers"}, {Method: "DELETE", Path: "/gtt/triggers/5"}, {Method: "POST", Path: "/alerts"}}
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/kite/mock/ -run 'TestFullSessionCycle|TestIsWrite' -v`
Expected: FAIL to compile with `undefined: mock.NewRecorder`.

- [ ] **Step 3: Write the recorder**

```go
// internal/kite/mock/recorder.go
package mock

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// Request is one call the app made to the fake Kite API.
type Request struct {
	Method string
	Path   string
	Form   url.Values
}

// Recorder is a recording Kite HTTP fake: canned envelopes per route, 404 GeneralException otherwise.
type Recorder struct {
	srv    *httptest.Server
	mu     sync.Mutex
	reqs   []Request
	routes map[string]route
}

type route struct {
	status int
	body   string
}

func NewRecorder(t testing.TB) *Recorder {
	r := &Recorder{routes: map[string]route{}}
	r.Route(http.MethodPost, "/session/token", 200, `{"status":"success","data":{"user_id":"AB1234","user_name":"Asha Bhat","api_key":"k","access_token":"acc-mock-0001"}}`)
	r.Route(http.MethodDelete, "/session/token", 200, `{"status":"success","data":true}`)
	r.Route(http.MethodGet, "/user/profile", 200, `{"status":"success","data":{"user_id":"AB1234","user_name":"Asha Bhat","email":"asha@example.com","broker":"ZERODHA","exchanges":["NSE","BSE"],"products":["CNC","MIS"]}}`)
	r.Route(http.MethodGet, "/instruments/historical/256265/day", 200, `{"status":"success","data":{"candles":[]}}`)
	r.Route(http.MethodGet, "/orders", 200, `{"status":"success","data":[]}`)
	r.Route(http.MethodGet, "/portfolio/positions", 200, `{"status":"success","data":{"net":[],"day":[]}}`)
	r.Route(http.MethodGet, "/portfolio/holdings", 200, `{"status":"success","data":[]}`)
	r.Route(http.MethodGet, "/quote/ltp", 200, `{"status":"success","data":{}}`)
	r.srv = httptest.NewServer(http.HandlerFunc(r.serve))
	t.Cleanup(r.srv.Close)
	return r
}

func (r *Recorder) Route(method, path string, status int, body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[method+" "+path] = route{status, body}
}

func (r *Recorder) URL() string          { return r.srv.URL }
func (r *Recorder) Client() *http.Client { return r.srv.Client() }

func (r *Recorder) Requests() []Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Request(nil), r.reqs...)
}

func (r *Recorder) serve(w http.ResponseWriter, req *http.Request) {
	_ = req.ParseForm()
	r.mu.Lock()
	r.reqs = append(r.reqs, Request{Method: req.Method, Path: req.URL.Path, Form: req.Form})
	rt, ok := r.routes[req.Method+" "+req.URL.Path]
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"status":"error","error_type":"GeneralException","message":"route not mocked"}`)
		return
	}
	w.WriteHeader(rt.status)
	_, _ = io.WriteString(w, rt.body)
}

// IsWrite reports whether a request could change account state at Kite. Only the login exchange and logout are allowed.
func IsWrite(req Request) bool {
	if req.Method == http.MethodGet {
		return false
	}
	return !(req.Path == "/session/token" && (req.Method == http.MethodPost || req.Method == http.MethodDelete))
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test -tags webkit2_41,noorders -race ./internal/kite/mock/ -run 'TestFullSessionCycle|TestIsWrite' -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Verify G-0 on the built artefact (ARCH §2.5 check 2)**

Run:
```bash
out=$(mktemp -d)/tradeloom && go build -tags webkit2_41,noorders -o "$out" ./cmd/tradeloom && \
if go tool nm -sort name "$out" | grep -E 'gokiteconnect/v4\.\(\*Client\)\.(PlaceOrder|ModifyOrder|CancelOrder|ExitOrder|ConvertPosition|PlaceGTT|ModifyGTT|DeleteGTT)$'; then echo "G-0 FAILED"; exit 1; else echo "G-0 OK"; fi
```
Expected: `G-0 OK` (no symbol lines printed).

- [ ] **Step 6: Commit**

```bash
git add internal/kite/mock/recorder.go internal/kite/mock/zero_writes_test.go
git commit -m "test(kite): full session cycle sends no order-write requests (G-0)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
