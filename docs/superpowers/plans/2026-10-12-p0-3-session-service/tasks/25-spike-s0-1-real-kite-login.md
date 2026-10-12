> **P0-3 Session & Service Layer Implementation Plan — Task 25.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 25: Spike S0-1 — real Kite login procedure and `docs/spec/kite-login.md`

**Files:**
- Create: `internal/kite/live_login_test.go` (`//go:build kitelive`), `docs/spec/kite-login.md`
- Modify (only if an observation contradicts an assumption): `internal/kite/probe.go` + `probe_test.go`, `internal/kite/login.go` + `login_test.go`, `internal/kite/credentials.go`

**Interfaces:**
- Consumes: `startLoopback`, `newNonce`, `NewAuth`, `ParseRedirect`, `DialTicker`, `NiftyIndexToken`, `MaskUserID`; gokiteconnect `GetHistoricalData`, `InvalidateAccessToken`.
- Produces: `docs/spec/kite-login.md` (decides OQ-07); possibly additional test variants in Tasks 7 and 9.

S0-1 settles the facts no primary source could:
- Does the developer console accept `http://127.0.0.1:7420/kite/callback`?
- Is `state` echoed through `redirect_params`?
- Which query keys arrive?
- What error does a reused or expired `request_token` return?
- What does a Personal-plan key get from historical and from the ticker?
- How long are API keys and secrets?

- [ ] **Step 1: Write the live harness**

```go
//go:build kitelive

// internal/kite/live_login_test.go — run by a developer against a real Kite account; never in CI.
package kite

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

func logKiteErr(t *testing.T, what string, err error) {
	t.Helper()
	var ke kiteconnect.Error
	switch {
	case err == nil:
		t.Logf("%s: OK", what)
	case errors.As(err, &ke):
		t.Logf("%s: error_type=%q http=%d message=%q", what, ke.ErrorType, ke.Code, ke.Message)
	default:
		t.Logf("%s: transport error %T", what, err)
	}
}

func TestLiveLogin(t *testing.T) {
	key, secret := os.Getenv("TRADELOOM_KITE_API_KEY"), os.Getenv("TRADELOOM_KITE_API_SECRET")
	if key == "" || secret == "" {
		t.Skip("set TRADELOOM_KITE_API_KEY and TRADELOOM_KITE_API_SECRET")
	}
	t.Logf("api key length %d, secret length %d", len(key), len(secret))
	port := DefaultRedirectPort
	if p := os.Getenv("TRADELOOM_KITE_PORT"); p != "" {
		port, _ = strconv.Atoi(p)
	}
	got := make(chan *http.Request, 1)
	lb, err := startLoopback(port, func(r *http.Request) (int, string) {
		got <- r.Clone(context.Background())
		return http.StatusOK, resultPage(pageTitleOK, pageMsgOK)
	}, slog.Default())
	if err != nil {
		t.Fatalf("listen on 127.0.0.1:%d: %v", port, err)
	}
	defer lb.Close()
	nonce, _ := newNonce()
	auth := NewAuth(&http.Client{Timeout: 10 * time.Second}, "")(key)
	t.Logf("Open this URL in a browser and log in:\n%s", auth.LoginURL(nonce))

	var r *http.Request
	select {
	case r = <-got:
	case <-time.After(5 * time.Minute):
		t.Fatal("no redirect within 5 minutes (record: loopback redirect not delivered)")
	}
	q := r.URL.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t.Logf("redirect query keys: %v; status=%q action=%q; state echoed: %v", keys, q.Get("status"), q.Get("action"), q.Get("state") == nonce)
	red, err := ParseRedirect("?" + r.URL.RawQuery)
	if err != nil {
		t.Fatalf("ParseRedirect: %v", err)
	}
	t.Logf("request_token length %d", len(red.RequestToken))

	if d := os.Getenv("TRADELOOM_KITE_DELAY_EXCHANGE"); d != "" {
		secs, _ := strconv.Atoi(d)
		t.Logf("waiting %d s before the exchange (request_token expiry probe)", secs)
		time.Sleep(time.Duration(secs) * time.Second)
	}
	sess, err := auth.GenerateSession(context.Background(), red.RequestToken, secret)
	logKiteErr(t, "first exchange", err)
	if err != nil {
		return
	}
	t.Logf("user %s, access_token length %d", MaskUserID(sess.UserID), len(sess.AccessToken))
	_, err = auth.GenerateSession(context.Background(), red.RequestToken, secret)
	logKiteErr(t, "second exchange of the same request_token", err)

	kc := kiteconnect.New(key)
	kc.SetAccessToken(sess.AccessToken)
	now := time.Now()
	_, err = kc.GetHistoricalData(NiftyIndexToken, "day", now.AddDate(0, 0, -7), now, false, false)
	logKiteErr(t, "historical probe (NIFTY 50 day)", err)
	t.Logf("IsNoDataPermission(historical error) = %v", IsNoDataPermission(err))
	err = DialTicker(context.Background(), nil, key, sess.AccessToken, 5*time.Second)
	t.Logf("ticker probe: %v", err)
	ok, err := kc.InvalidateAccessToken()
	logKiteErr(t, "InvalidateAccessToken", err)
	t.Logf("invalidated: %v", ok)
}
```

- [ ] **Step 2: Check that it compiles and skips without credentials**

Run: `go test -tags 'webkit2_41,noorders,kitelive' ./internal/kite/ -run TestLiveLogin -v`
Expected: `--- SKIP: TestLiveLogin (… set TRADELOOM_KITE_API_KEY and TRADELOOM_KITE_API_SECRET)`.

- [ ] **Step 3: Write `docs/spec/kite-login.md`**

````markdown
# Kite login — S0-1 procedure and findings

Status: procedure ready; findings recorded on the date in each table row. Decides OQ-07 (ARCH §12.1).

## 1. Register the Kite Connect app

1. Open https://developers.kite.trade → Create new app. Type: Connect (data) or Personal (no data; second run).
2. Redirect URL: `http://127.0.0.1:7420/kite/callback` (Tradeloom's default; Settings → Kite connection shows the exact value).
3. Postback URL: leave empty (Tradeloom does not use postbacks, FR-ORD-014).
4. Screenshot the form after saving (or the validation error) into `docs/spec/img/s0-1-console.png`.

## 2. Run the harness (developer shell, never CI)

```bash
TRADELOOM_KITE_API_KEY=… TRADELOOM_KITE_API_SECRET=… \
  go test -tags 'webkit2_41,noorders,kitelive' -run TestLiveLogin -v -timeout 10m ./internal/kite/
```

Open the printed URL, log in (password + 2FA), and copy the `-v` log lines into §3. Repeat with:
- `TRADELOOM_KITE_DELAY_EXCHANGE=300` (request_token expiry);
- a Personal-plan key (degraded mode);
- `TRADELOOM_KITE_PORT=7421` after registering that port (configurable port).

The harness prints lengths, masked ids and error types only, never tokens or the secret.

## 3. Findings

| # | Question | How observed | Result (fill from the harness log) | Date |
|---|---|---|---|---|
| 1 | Console accepts `http://127.0.0.1:7420/kite/callback` | §1 screenshot | | |
| 2 | Browser reaches the loopback listener after login | harness "redirect query keys" line | | |
| 3 | Query keys on the redirect | same line | | |
| 4 | `state` echoed via `redirect_params` | "state echoed" | | |
| 5 | Second exchange of the same request_token | "second exchange" line | | |
| 6 | Exchange after 300 s | DELAY run, "first exchange" line | | |
| 7 | Personal key: historical probe error | "historical probe" line | | |
| 8 | Personal key: `IsNoDataPermission` | next line | | |
| 9 | Personal key: ticker probe | "ticker probe" line | | |
| 10 | API key / secret / request_token lengths | first lines | | |
| 11 | NIFTY 50 token 256265 returns data on Connect | "historical probe" = OK | | |

## 4. Decision rules (apply in the same PR as the findings)

- Row 1 or 2 negative → OQ-07 = paste primary. P0-4 shows the paste field first; the loopback code stays (ARCH §13.3).
- Row 4 negative → no code change: callbacks without `state` are accepted only while a login is pending (Task 7).
- Rows 5/6 not `TokenException` → add the observed `error_type`/message to `mapExchangeError` and a case to `TestMapExchangeError`.
- Rows 7/8: if `IsNoDataPermission` is false → add the observed error to `TestIsNoDataPermission_Variants` and extend `IsNoDataPermission` until it passes.
- Row 9 shows the ticker connecting on a Personal key → no change (historical decides the plan).
- Row 10 shows uniform lengths → tighten `credRe` in `credentials.go` and its test table.
- Row 11 negative → replace `NiftyIndexToken` with the observed index token.
````

- [ ] **Step 4: Run the spike with a real account**

Perform §1 and §2 of the document, fill §3, apply §4, and run `go test -tags webkit2_41,noorders -race ./internal/kite/... -v`.
Expected: every `internal/kite` test passes, including any new variants added by §4.

- [ ] **Step 5: Commit**

```bash
git add internal/kite/live_login_test.go docs/spec/kite-login.md docs/spec/img internal/kite
git commit -m "spike(S0-1): real Kite login harness and findings (OQ-07)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
