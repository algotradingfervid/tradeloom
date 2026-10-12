> **P0-3 Session & Service Layer Implementation Plan — Task 7.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 7: Login flow — loopback with state, paste fallback, token exchange

**Files:**
- Create: `internal/kite/login.go`
- Test: `internal/kite/login_test.go`

**Interfaces:**
- Consumes: Tasks 3, 4 and 6.
- Produces:
  ```go
  type LoginStart struct{ LoginURL, RedirectURL string; ListenerOK bool; ListenerError string } // TS kite.LoginStart
  func (s *Session) BeginLogin(ctx context.Context) (LoginStart, error)
  func (s *Session) CompleteLogin(ctx context.Context, pasted string) (SessionState, error) // paste fallback (FR-ACCT-006 AC2)
  func (s *Session) CancelLogin() SessionState
  func mapExchangeError(err error) *LoginError
  // test helper in login_test.go, used by Tasks 8, 10, 11: func loginViaPaste(t *testing.T, e *testEnv) SessionState
  ```

Rules:
- A callback whose `state` was not issued in this run is refused, and the session is left untouched. A callback without `state` is accepted only while a login is pending.
- A pasted URL is accepted in any non-ready state, whatever its `state`. Pasting is an explicit user action.
- A request token already exchanged in this run returns the success page again (browser reload).
- `TokenException` from `GenerateSession` maps to `request_token_invalid`. It never goes through `HandleTokenError`.
- The listener stays up for its full 10 minutes so reloads keep working. The timeout only resets the state if the login is still pending.

- [ ] **Step 1: Write the failing tests**

```go
// internal/kite/login_test.go
package kite

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
)

func setPort(t *testing.T, e *testEnv, port int) {
	t.Helper()
	if err := settings.Set(context.Background(), e.settings, SettingRedirectPort, port); err != nil {
		t.Fatal(err)
	}
}

func stateFrom(t *testing.T, loginURL string) string {
	t.Helper()
	u, err := url.Parse(loginURL)
	if err != nil {
		t.Fatal(err)
	}
	v, err := url.ParseQuery(u.Query().Get("redirect_params"))
	if err != nil {
		t.Fatal(err)
	}
	return v.Get("state")
}

func httpGet(t *testing.T, u string) (int, string) {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// loginViaPaste logs in with request token "rtok12345678" and waits for the feed to start.
func loginViaPaste(t *testing.T, e *testEnv) SessionState {
	t.Helper()
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	if _, err := e.sess.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, err := e.sess.CompleteLogin(context.Background(), "rtok12345678")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "feed started", func() bool { return e.feed.startCount() == 1 })
	return st
}

// FR-ACCT-006 AC1 (backend).
func TestLogin_LoopbackCallbackCompletesLogin(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	port := freePort(t)
	setPort(t, e, port)
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ls.ListenerOK || ls.RedirectURL != RedirectURL(port) || e.sess.State().State != StateLoggingIn {
		t.Fatalf("start %+v state %s", ls, e.sess.State().State)
	}
	state := stateFrom(t, ls.LoginURL)
	if len(state) < 20 {
		t.Fatalf("state nonce %q too short", state)
	}
	code, body := httpGet(t, ls.RedirectURL+"?request_token=rtok12345678&action=login&status=success&state="+state)
	if code != http.StatusOK || !strings.Contains(body, "Logged in — you can close this browser tab") {
		t.Fatalf("callback %d %s", code, body)
	}
	st := e.sess.State()
	if st.State != StateReady || !st.RiskAckRequired || st.ExpiresAtMs != ist(2026, 10, 13, 6, 0).UnixMilli() {
		t.Fatalf("state %+v", st)
	}
	raw, _ := e.secrets.Get(secrets.KeyAccessToken)
	var rec TokenRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil || rec.AccessToken != "acc-live-777" {
		t.Fatalf("token record %q", raw)
	}
	if e.tokens.get() != "acc-live-777" || e.lim.isPaused() {
		t.Fatal("token not applied or limiter not resumed")
	}
	eventually(t, "feed started", func() bool { return e.feed.startCount() == 1 })
	if row, err := e.repo.Get(context.Background(), st.SessionID); err != nil || row.UserID != "AB1234" {
		t.Fatalf("row %+v %v", row, err)
	}
	if strings.Contains(e.logs.String(), "acc-live-777") || strings.Contains(e.logs.String(), "rtok12345678") {
		t.Fatal("token leaked into logs")
	}
}

// FR-ACCT-006 AC2: the listener cannot bind; pasting the redirect URL completes the login.
func TestLogin_PasteWhenListenerBlocked(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	setPort(t, e, port)
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ls.ListenerOK || !strings.Contains(ls.ListenerError, "paste") {
		t.Fatalf("start %+v", ls)
	}
	st, err := e.sess.CompleteLogin(context.Background(), fmt.Sprintf("http://127.0.0.1:%d/kite/callback?request_token=rtok12345678&status=success", port))
	if err != nil || st.State != StateReady {
		t.Fatalf("st %+v err %v", st, err)
	}
}

// Review Focus 1.
func TestLogin_AfterListenerTimeout_PasteStillWorks(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	state := stateFrom(t, ls.LoginURL)
	e.clock.Advance(10*time.Minute + time.Second)
	eventually(t, "listener timeout", func() bool {
		st := e.sess.State()
		return st.State == StateLoggedOut && st.ErrorCode == CodeListenerTimeout
	})
	eventually(t, "listener closed", func() bool {
		resp, err := http.Get(ls.RedirectURL)
		if err == nil {
			resp.Body.Close()
		}
		return err != nil
	})
	st, err := e.sess.CompleteLogin(context.Background(), ls.RedirectURL+"?request_token=rtok12345678&status=success&state="+state)
	if err != nil || st.State != StateReady {
		t.Fatalf("st %+v err %v", st, err)
	}
}

// Review Focus 2: reloading the callback tab after success.
func TestLogin_CallbackReloadAfterSuccess_IsIdempotent(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	ls, _ := e.sess.BeginLogin(context.Background())
	u := ls.RedirectURL + "?request_token=rtok12345678&status=success&state=" + stateFrom(t, ls.LoginURL)
	if code, _ := httpGet(t, u); code != http.StatusOK {
		t.Fatalf("first %d", code)
	}
	code, body := httpGet(t, u)
	if code != http.StatusOK || !strings.Contains(body, "Logged in") {
		t.Fatalf("reload %d %s", code, body)
	}
	if e.auth.exchangeCount() != 1 {
		t.Fatalf("exchanged %d times", e.auth.exchangeCount())
	}
}

// Review Focus 2: a reused or expired request_token.
func TestLogin_RequestTokenReusedOrExpired(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	pausesBefore, resumesBefore := e.lim.counts()
	_, err = e.sess.CompleteLogin(context.Background(), "rtokexpired99")
	var le *LoginError
	if !errors.As(err, &le) || le.Code != CodeRequestTokenInvalid || !strings.Contains(le.Message, "Log in with Kite") {
		t.Fatalf("err %v", err)
	}
	st := e.sess.State()
	if st.State != StateLoggedOut || st.ErrorCode != CodeRequestTokenInvalid {
		t.Fatalf("state %+v (must not be expired)", st)
	}
	if p, r := e.lim.counts(); p != pausesBefore || r != resumesBefore {
		t.Fatal("a failed exchange must not touch the limiter")
	}
	code, body := httpGet(t, ls.RedirectURL+"?request_token=rtokexpired99&state="+stateFrom(t, ls.LoginURL))
	if code != http.StatusBadRequest || !strings.Contains(body, "already used or has expired") {
		t.Fatalf("callback %d %s", code, body)
	}
}

func TestLogin_CallbackWithForeignState_Rejected(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	ls, _ := e.sess.BeginLogin(context.Background())
	code, body := httpGet(t, ls.RedirectURL+"?request_token=rtok12345678&state=forgedstate")
	if code != http.StatusBadRequest || !strings.Contains(body, "not recognised") {
		t.Fatalf("%d %s", code, body)
	}
	if e.auth.exchangeCount() != 0 || e.sess.State().State != StateLoggingIn {
		t.Fatal("a foreign state must not exchange or change state")
	}
}

func TestBeginLogin_RequiresCredentialsAndCancelRestores(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	_, err := e.sess.BeginLogin(context.Background())
	var le *LoginError
	if !errors.As(err, &le) || le.Code != CodeNoCredentials {
		t.Fatalf("err %v", err)
	}
	e.configure()
	setPort(t, e, freePort(t))
	ls, _ := e.sess.BeginLogin(context.Background())
	if st := e.sess.CancelLogin(); st.State != StateLoggedOut {
		t.Fatalf("cancel -> %s", st.State)
	}
	eventually(t, "listener closed", func() bool {
		resp, err := http.Get(ls.RedirectURL)
		if err == nil {
			resp.Body.Close()
		}
		return err != nil
	})
}

func TestKcAuth_ChecksumLoginURLAndTokenException(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/session/token" {
			http.Error(w, "unexpected", http.StatusTeapot)
			return
		}
		_ = r.ParseForm()
		rt := r.PostForm.Get("request_token")
		want := fmt.Sprintf("%x", sha256.Sum256([]byte(testKey+rt+testSecret)))
		if r.PostForm.Get("checksum") != want || r.PostForm.Get("api_key") != testKey || r.PostForm.Get("api_secret") != "" {
			http.Error(w, `{"status":"error","error_type":"InputException","message":"Invalid checksum"}`, http.StatusBadRequest)
			return
		}
		if rt == "rtokused00000" {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `{"status":"error","error_type":"TokenException","message":"Token is invalid or has expired."}`)
			return
		}
		io.WriteString(w, `{"status":"success","data":{"user_id":"AB1234","access_token":"acc-1"}}`)
	}))
	defer srv.Close()
	auth := NewAuth(srv.Client(), srv.URL)(testKey)
	if got := auth.LoginURL("n1"); got != "https://kite.zerodha.com/connect/login?api_key="+testKey+"&v=3&redirect_params=state%3Dn1" {
		t.Fatalf("login URL %s", got)
	}
	s, err := auth.GenerateSession(context.Background(), "rtok12345678", testSecret)
	if err != nil || s.AccessToken != "acc-1" || s.UserID != "AB1234" {
		t.Fatalf("%+v %v", s, err)
	}
	_, err = auth.GenerateSession(context.Background(), "rtokused00000", testSecret)
	if le := mapExchangeError(err); le.Code != CodeRequestTokenInvalid {
		t.Fatalf("mapped %+v", le)
	}
}

func TestMapExchangeError(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{APIError(kiteconnect.TokenError, 403, "Token is invalid or has expired."), CodeRequestTokenInvalid},
		{APIError(kiteconnect.InputError, 400, "Invalid checksum"), CodeCredentialsRejected},
		{APIError(kiteconnect.NetworkError, 503, "Request failed."), CodeNetwork},
		{APIError(kiteconnect.GeneralError, 500, "Something broke"), CodeKiteError},
		{errors.New("dial tcp: connection refused"), CodeNetwork},
	}
	for _, c := range cases {
		if got := mapExchangeError(c.err); got.Code != c.code {
			t.Fatalf("%v -> %s, want %s", c.err, got.Code, c.code)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestLogin|TestBeginLogin|TestKcAuth|TestMapExchangeError' -v`
Expected: FAIL to compile with `undefined: (*Session).BeginLogin`, `undefined: mapExchangeError`.

- [ ] **Step 3: Write the implementation**

```go
// internal/kite/login.go
package kite

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/bus"
	"tradeloom/internal/logging"
	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

// LoginStart tells the UI what to open and whether the loopback listener is up.
type LoginStart struct {
	LoginURL      string `json:"loginUrl"`
	RedirectURL   string `json:"redirectUrl"`
	ListenerOK    bool   `json:"listenerOk"`
	ListenerError string `json:"listenerError"`
}

func newNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// BeginLogin starts (or restarts) a login attempt: a fresh state nonce, the loopback listener and the login URL.
// The caller opens LoginURL in the system browser (internal/app does it with runtime.BrowserOpenURL).
func (s *Session) BeginLogin(_ context.Context) (LoginStart, error) {
	if !s.configured() {
		return LoginStart{}, &LoginError{Code: CodeNoCredentials, Message: "Set up your Kite API key and secret first."}
	}
	apiKey := s.apiKey()
	port := settings.Get(s.d.Settings, SettingRedirectPort)
	nonce, err := newNonce()
	if err != nil {
		return LoginStart{}, err
	}
	start := LoginStart{LoginURL: s.d.Auth(apiKey).LoginURL(nonce), RedirectURL: RedirectURL(port)}

	s.mu.Lock()
	if s.st.State == StateReady {
		s.mu.Unlock()
		return LoginStart{}, &LoginError{Code: CodeAlreadyLoggedIn, Message: "Already logged in to Kite."}
	}
	if s.st.State != StateLoggingIn {
		s.prior = s.st.State
	}
	s.nonces[nonce] = true
	if s.lb != nil && s.lb.port != port {
		s.lb.Close()
		s.lb = nil
	}
	if s.lb == nil {
		lb, lerr := startLoopback(port, s.handleCallback, s.d.Log)
		if lerr != nil {
			start.ListenerError = listenerHint(port, lerr)
		} else {
			s.lb = lb
		}
	}
	start.ListenerOK = s.lb != nil
	s.gen++
	gen := s.gen
	st := SessionState{State: StateLoggingIn, DataPlan: PlanUnknown}
	s.st = st
	runCtx := s.runCtx
	s.mu.Unlock()

	bus.Publish(s.d.Bus, TopicSessionState, st)
	timeout := s.d.Clock.At(s.d.Clock.Now().Add(s.d.ListenerTimeout))
	logging.Go("kite.login.timeout", func() {
		select {
		case <-runCtx.Done():
			timeout.Stop()
		case <-timeout.C():
			s.onLoginTimeout(gen)
		}
	})
	s.d.Log.Info("kite login started", "listener", start.ListenerOK, "port", port)
	return start, nil
}

func (s *Session) onLoginTimeout(gen int) {
	s.mu.Lock()
	if gen != s.gen {
		s.mu.Unlock()
		return
	}
	lb := s.lb
	s.lb = nil
	pending := s.st.State == StateLoggingIn
	st := SessionState{State: s.prior, DataPlan: PlanUnknown, ErrorCode: CodeListenerTimeout,
		ErrorMessage: "The login window timed out. If you already logged in, paste the address from the browser's address bar below; otherwise click \"Log in with Kite\" again."}
	if pending {
		s.st = st
	}
	s.mu.Unlock()
	if lb != nil {
		lb.Close()
	}
	if pending {
		bus.Publish(s.d.Bus, TopicSessionState, st)
	}
}

// CancelLogin abandons a pending login and closes the listener.
func (s *Session) CancelLogin() SessionState {
	s.mu.Lock()
	if s.st.State != StateLoggingIn {
		st := s.st
		s.mu.Unlock()
		return st
	}
	s.gen++
	lb := s.lb
	s.lb = nil
	st := SessionState{State: s.prior, DataPlan: PlanUnknown}
	s.st = st
	s.mu.Unlock()
	if lb != nil {
		lb.Close()
	}
	bus.Publish(s.d.Bus, TopicSessionState, st)
	return st
}

func (s *Session) handleCallback(r *http.Request) (int, string) {
	red, err := ParseRedirect("?" + r.URL.RawQuery)
	if err != nil {
		return http.StatusBadRequest, resultPage("Login not completed", err.Error())
	}
	s.mu.Lock()
	already := s.done[red.RequestToken]
	known := red.State != "" && s.nonces[red.State]
	pending := s.st.State == StateLoggingIn
	ctx := s.runCtx
	s.mu.Unlock()
	switch {
	case already:
		return http.StatusOK, resultPage(pageTitleOK, pageMsgOK)
	case red.State != "" && !known:
		return http.StatusBadRequest, resultPage("Login link not recognised",
			"This login was started by an earlier Tradeloom run or another app. Return to Tradeloom and click \"Log in with Kite\" again.")
	case red.State == "" && !pending:
		return http.StatusBadRequest, resultPage("No login in progress", "Return to Tradeloom and click \"Log in with Kite\".")
	}
	if _, err := s.exchange(ctx, red.RequestToken); err != nil {
		return http.StatusBadRequest, resultPage("Login failed", err.Error())
	}
	return http.StatusOK, resultPage(pageTitleOK, pageMsgOK)
}

// CompleteLogin is the paste fallback: a full redirect URL, its query, or a bare request_token.
func (s *Session) CompleteLogin(ctx context.Context, pasted string) (SessionState, error) {
	red, err := ParseRedirect(pasted)
	if err != nil {
		return s.State(), err
	}
	if red.State != "" {
		s.mu.Lock()
		known := s.nonces[red.State]
		s.mu.Unlock()
		if !known {
			s.d.Log.Info("pasted redirect carries a state from another login attempt; accepted because pasting is explicit")
		}
	}
	return s.exchange(ctx, red.RequestToken)
}

func (s *Session) exchange(ctx context.Context, requestToken string) (SessionState, error) {
	s.exMu.Lock()
	defer s.exMu.Unlock()

	s.mu.Lock()
	if s.done[requestToken] {
		st := s.st
		s.mu.Unlock()
		return st, nil
	}
	if s.st.State == StateReady {
		st := s.st
		s.mu.Unlock()
		return st, &LoginError{Code: CodeAlreadyLoggedIn, Message: "Already logged in to Kite."}
	}
	s.mu.Unlock()

	apiKey := s.apiKey()
	secret, ok := s.secret()
	if apiKey == "" || !ok {
		return s.State(), &LoginError{Code: CodeNoCredentials, Message: "Set up your Kite API key and secret first."}
	}
	auth := s.d.Auth(apiKey)
	us, err := auth.GenerateSession(ctx, requestToken, secret)
	if err != nil {
		le := mapExchangeError(err)
		s.failLogin(le)
		return s.State(), le
	}
	logging.RegisterSecret(us.AccessToken)
	now := s.d.Clock.Now()
	rec := TokenRecord{AccessToken: us.AccessToken, UserID: us.UserID, SessionID: s.d.NewID(),
		LoggedInAtMs: now.UnixMilli(), ExpiresAtMs: NextExpiry(now).UnixMilli()}
	b, err := json.Marshal(rec)
	if err != nil {
		return s.State(), err
	}
	if err := s.d.Secrets.Set(secrets.KeyAccessToken, string(b)); err != nil {
		_ = auth.Invalidate(ctx, us.AccessToken)
		le := &LoginError{Code: CodeSecretStore, Message: "Logged in to Kite, but the access token could not be saved to the secret store: " + err.Error()}
		s.failLogin(le)
		return s.State(), le
	}
	if err := s.d.Repo.Insert(ctx, store.SessionRow{ID: rec.SessionID, UserID: rec.UserID, LoggedInAtMs: rec.LoggedInAtMs,
		ExpiresAtMs: rec.ExpiresAtMs, DataPlan: string(PlanUnknown)}); err != nil {
		s.d.Log.Error("recording the session failed", "err", err)
	}
	s.d.Tokens.SetAPIKey(apiKey)
	s.d.Tokens.SetAccessToken(rec.AccessToken)
	s.d.Limiter.Resume()
	s.mu.Lock()
	s.done[requestToken] = true
	s.rec = &rec
	s.mu.Unlock()
	st := s.set(readyState(rec, PlanUnknown, true, false))
	s.d.Log.Info("kite login complete", "user", MaskUserID(rec.UserID), "expires_at_ms", rec.ExpiresAtMs)
	s.afterReady(apiKey, rec)
	return st, nil
}

func (s *Session) failLogin(le *LoginError) {
	s.mu.Lock()
	base := s.st.State
	if base == StateLoggingIn || base == StateReady {
		base = s.prior
	}
	st := SessionState{State: base, DataPlan: PlanUnknown, ErrorCode: le.Code, ErrorMessage: le.Message}
	s.st = st
	s.mu.Unlock()
	bus.Publish(s.d.Bus, TopicSessionState, st)
	s.d.Log.Warn("kite login failed", "code", le.Code)
}

// mapExchangeError turns a GenerateSession failure into user guidance. A TokenException here means the
// request_token is spent or stale; it is NOT a session expiry (Review Focus 2).
func mapExchangeError(err error) *LoginError {
	var ke kiteconnect.Error
	if errors.As(err, &ke) {
		msg := strings.ToLower(ke.Message)
		switch {
		case ke.ErrorType == kiteconnect.TokenError:
			return &LoginError{Code: CodeRequestTokenInvalid, Message: "This login link was already used or has expired (Kite request tokens last only a few minutes). Click \"Log in with Kite\" again."}
		case ke.ErrorType == kiteconnect.InputError && (strings.Contains(msg, "checksum") || strings.Contains(msg, "api_key") || strings.Contains(msg, "api key")):
			return &LoginError{Code: CodeCredentialsRejected, Message: "Kite rejected the API key or secret (" + ke.Message + "). Check them in Settings → Kite connection."}
		case ke.ErrorType == kiteconnect.NetworkError:
			return &LoginError{Code: CodeNetwork, Message: "Could not reach Kite. Check the internet connection and try again."}
		default:
			return &LoginError{Code: CodeKiteError, Message: "Kite refused the login: " + ke.Message}
		}
	}
	return &LoginError{Code: CodeNetwork, Message: "Could not reach Kite. Check the internet connection and try again."}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/kite/ -run 'TestLogin|TestBeginLogin|TestKcAuth|TestMapExchangeError' -v`
Expected: PASS (9 tests; `-race` clean).

- [ ] **Step 5: Commit**

```bash
git add internal/kite/login.go internal/kite/login_test.go
git commit -m "feat(kite): loopback and paste login with state nonce and exchange error mapping

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
