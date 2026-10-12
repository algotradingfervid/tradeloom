> **P0-3 Session & Service Layer Implementation Plan — Task 9.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 9: Capability probe and degraded-mode detection

**Files:**
- Create: `internal/kite/probe.go`
- Test: `internal/kite/probe_test.go`

**Interfaces:**
- Consumes: gokiteconnect `ticker` package (`package kiteticker`, verified `ticker/ticker.go`): `New(apiKey, accessToken string) *Ticker`, `SetRootURL(url.URL)`, `SetAutoReconnect(bool)`, `SetReconnectMaxRetries(int)`, `SetConnectTimeout(time.Duration)`, `OnConnect(func())`, `OnError(func(error))`, `OnNoReconnect(func(int))`, `ServeWithContext(context.Context)`, `Stop()`, exported field `Conn *websocket.Conn`. P0-2's `ReadClient.Historical` signature.
- Produces:
  ```go
  const NiftyIndexToken = 256265
  type HistoricalReader interface{ Historical(ctx context.Context, token int, iv string, from, to time.Time, continuous, oi bool) ([]kiteconnect.HistoricalData, error) }
  type CapabilityProbe struct{ Historical func(ctx context.Context) error; TickerConnect func(ctx context.Context, apiKey, accessToken string) error; Log *slog.Logger }
  func NewCapabilityProbe(h HistoricalReader, clk clock.Clock, tickerRoot *url.URL, log *slog.Logger) *CapabilityProbe
  func (p *CapabilityProbe) Probe(ctx context.Context, apiKey, accessToken string) DataPlan // implements Prober
  func IsNoDataPermission(err error) bool
  func DialTicker(ctx context.Context, root *url.URL, apiKey, accessToken string, timeout time.Duration) error
  ```

Decision table (ARCH §5.2, FR-ACCT-007). Nothing in it can place anything: one `GET /instruments/historical/256265/day` and, only when that is ambiguous, one WebSocket handshake.

| Historical result | Ticker attempt | DataPlan |
|---|---|---|
| success | not needed (the feed starts next and is itself the ticker attempt) | `connect` |
| no-data permission error | skipped | `none` |
| anything else (network, 5xx, `TokenException`) | connects | `connect` |
| anything else | fails | `unknown` (Task 6 retries up to 3× every 30 s, then starts the feed optimistically) |

Kite's exception page lists no `PermissionException`, gokiteconnect names its constant `PermissionError`, and the real response for a Personal key is unverified (S0-1 records it). `IsNoDataPermission` therefore accepts all of these: either spelling; any HTTP 403 that is not a `TokenException`; any non-token error whose message contains "permission".

`DialTicker` uses `SetAutoReconnect(true)` + `SetReconnectMaxRetries(0)` on purpose. In v4.4.3, `ServeWithContext` with auto-reconnect off falls through a failed `Dial` into `triggerConnect()` and `t.Conn.SetCloseHandler` on a nil connection, which would report a false success and then panic.

- [ ] **Step 1: Write the failing tests**

```go
// internal/kite/probe_test.go
package kite

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	kiteconnect "github.com/zerodha/gokiteconnect/v4"
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestIsNoDataPermission|TestProbe|TestDialTicker' -v`
Expected: FAIL to compile with `undefined: IsNoDataPermission`, `undefined: CapabilityProbe`, `undefined: DialTicker`.

- [ ] **Step 3: Write the implementation**

```go
// internal/kite/probe.go
package kite

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	kiteticker "github.com/zerodha/gokiteconnect/v4/ticker"

	"tradeloom/internal/clock"
	"tradeloom/internal/logging"
)

// NiftyIndexToken is the NIFTY 50 index instrument token in Kite's instrument dump (S0-1 re-checks it).
const NiftyIndexToken = 256265

const probeTickerTimeout = 5 * time.Second

type HistoricalReader interface {
	Historical(ctx context.Context, token int, iv string, from, to time.Time, continuous, oi bool) ([]kiteconnect.HistoricalData, error)
}

// CapabilityProbe decides Connect vs Personal plan without placing anything (FR-ACCT-007).
type CapabilityProbe struct {
	Historical    func(ctx context.Context) error
	TickerConnect func(ctx context.Context, apiKey, accessToken string) error
	Log           *slog.Logger
}

func NewCapabilityProbe(h HistoricalReader, clk clock.Clock, tickerRoot *url.URL, log *slog.Logger) *CapabilityProbe {
	return &CapabilityProbe{
		Historical: func(ctx context.Context) error {
			now := clk.Now()
			_, err := h.Historical(ctx, NiftyIndexToken, "day", now.AddDate(0, 0, -7), now, false, false)
			return err
		},
		TickerConnect: func(ctx context.Context, apiKey, accessToken string) error {
			return DialTicker(ctx, tickerRoot, apiKey, accessToken, probeTickerTimeout)
		},
		Log: log,
	}
}

func (p *CapabilityProbe) Probe(ctx context.Context, apiKey, accessToken string) DataPlan {
	err := p.Historical(ctx)
	switch {
	case err == nil:
		return PlanConnect
	case IsNoDataPermission(err):
		p.Log.Info("capability probe: no historical-data permission", "kind", errKind(err))
		return PlanNone
	}
	p.Log.Warn("capability probe: historical request inconclusive; trying the ticker", "kind", errKind(err))
	if terr := p.TickerConnect(ctx, apiKey, accessToken); terr == nil {
		return PlanConnect
	}
	return PlanUnknown
}

// IsNoDataPermission recognises every observed spelling of "this API key has no market-data permission".
func IsNoDataPermission(err error) bool {
	var ke kiteconnect.Error
	if !errors.As(err, &ke) {
		return false
	}
	if ke.ErrorType == kiteconnect.TokenError {
		return false
	}
	if ke.ErrorType == kiteconnect.PermissionError || ke.ErrorType == "PermissionException" || ke.Code == 403 {
		return true
	}
	return strings.Contains(strings.ToLower(ke.Message), "permission")
}

// DialTicker opens one WebSocket connection and closes it again. It returns nil when the handshake succeeded.
func DialTicker(ctx context.Context, root *url.URL, apiKey, accessToken string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout+2*time.Second)
	defer cancel()
	tk := kiteticker.New(apiKey, accessToken)
	if root != nil {
		tk.SetRootURL(*root)
	}
	// Auto-reconnect with zero retries: see the note above (v4.4.3 nil-conn path when auto-reconnect is off).
	tk.SetAutoReconnect(true)
	tk.SetReconnectMaxRetries(0)
	tk.SetConnectTimeout(timeout)

	var (
		mu      sync.Mutex
		lastErr error
	)
	res := make(chan error, 1)
	tk.OnError(func(err error) {
		mu.Lock()
		lastErr = err
		mu.Unlock()
	})
	tk.OnConnect(func() {
		conn := tk.Conn // set by ServeWithContext just before this callback, same goroutine
		tk.Stop()
		if conn != nil {
			_ = conn.Close()
		}
		select {
		case res <- nil:
		default:
		}
	})
	tk.OnNoReconnect(func(int) {
		mu.Lock()
		err := lastErr
		mu.Unlock()
		if err == nil {
			err = errors.New("ticker connection failed")
		}
		select {
		case res <- err:
		default:
		}
	})
	logging.Go("kite.probe.ticker", func() { tk.ServeWithContext(ctx) })
	select {
	case err := <-res:
		return err
	case <-ctx.Done():
		tk.Stop()
		return fmt.Errorf("ticker connect timed out: %w", ctx.Err())
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/kite/ -run 'TestIsNoDataPermission|TestProbe|TestDialTicker' -v`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/kite/probe.go internal/kite/probe_test.go go.mod go.sum
git commit -m "feat(kite): capability probe for Connect vs Personal plan

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
