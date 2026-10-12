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

// Ruling 36: the loopback callback requires a state nonce issued in this run; a stateless GET is a
// forged local request and is refused, while pasting the same stateless redirect still logs in.
func TestLogin_CallbackWithoutState_Rejected(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	code, body := httpGet(t, ls.RedirectURL+"?request_token=rtok12345678&status=success")
	if code != http.StatusBadRequest || !strings.Contains(body, "not recognised") {
		t.Fatalf("%d %s", code, body)
	}
	if e.auth.exchangeCount() != 0 || e.sess.State().State != StateLoggingIn {
		t.Fatal("a stateless callback must not exchange or change state")
	}
	st, err := e.sess.CompleteLogin(context.Background(), ls.RedirectURL+"?request_token=rtok12345678&status=success")
	if err != nil || st.State != StateReady {
		t.Fatalf("paste: st %+v err %v", st, err)
	}
}

// Task 4 carry-forward: CSP on every callback page, idle and header limits on the server.
func TestLogin_LoopbackHardening(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(ls.RedirectURL + "?request_token=rtok12345678&state=" + stateFrom(t, ls.LoginURL))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'unsafe-inline'" {
		t.Fatalf("CSP %q", got)
	}
	e.sess.mu.Lock()
	srv := e.sess.lb.srv
	e.sess.mu.Unlock()
	if srv.IdleTimeout != 30*time.Second || srv.MaxHeaderBytes != 8<<10 {
		t.Fatalf("idle %v maxHeader %d", srv.IdleTimeout, srv.MaxHeaderBytes)
	}
}

// Task 3 carry-forward: a non-success status is never shown untruncated (callback page or paste error).
func TestLogin_CancelledStatusNotEchoedUntruncated(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("Z", 300)
	code, body := httpGet(t, ls.RedirectURL+"?status="+long+"&state="+stateFrom(t, ls.LoginURL))
	if code != http.StatusBadRequest || strings.Contains(body, strings.Repeat("Z", 41)) || !strings.Contains(body, "not completed") {
		t.Fatalf("%d %s", code, body)
	}
	_, err = e.sess.CompleteLogin(context.Background(), "http://127.0.0.1/kite/callback?status="+long)
	var le *LoginError
	if !errors.As(err, &le) || le.Code != CodeLoginCancelled || strings.Contains(le.Message, strings.Repeat("Z", 41)) {
		t.Fatalf("err %v", err)
	}
	if e.auth.exchangeCount() != 0 {
		t.Fatal("a cancelled redirect must not exchange")
	}
}

type echoAuth struct{ fakeAuth }

func (a *echoAuth) GenerateSession(_ context.Context, rt, secret string) (AuthSession, error) {
	return AuthSession{}, APIError(kiteconnect.InputError, 400, "Invalid checksum for api_key "+testKey+" token "+rt+" secret "+secret)
}

// An exchange error never echoes the request token or the API secret (returned error, state, page, logs).
func TestLogin_ExchangeErrorNeverEchoesTokenOrSecret(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	ea := &echoAuth{}
	e.sess.d.Auth = func(string) AuthAPI { return ea }
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.sess.CompleteLogin(context.Background(), "rtokleaky1234")
	var le *LoginError
	if !errors.As(err, &le) || le.Code != CodeCredentialsRejected {
		t.Fatalf("err %v", err)
	}
	_, body := httpGet(t, ls.RedirectURL+"?request_token=rtokleaky5678&state="+stateFrom(t, ls.LoginURL))
	for _, s := range []string{le.Message, e.sess.State().ErrorMessage, body, e.logs.String()} {
		if strings.Contains(s, "rtokleaky") || strings.Contains(s, testSecret) {
			t.Fatalf("leaked: %s", s)
		}
	}
}

type failSetSecrets struct{ *memSecrets }

func (f failSetSecrets) Set(k secrets.Key, v string) error {
	if k == secrets.KeyAccessToken {
		return fmt.Errorf("%w: keyring locked", secrets.ErrUnavailable)
	}
	return f.memSecrets.Set(k, v)
}

// Ruling 52 (6): a token that cannot be stored gives the fixed secret-store message, and the
// just-issued access token is invalidated at Kite.
func TestLogin_TokenStoreFailure_FixedMessageAndInvalidates(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	e.sess.d.Secrets = newBoundedSecrets(failSetSecrets{e.secrets})
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	if _, err := e.sess.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := e.sess.CompleteLogin(context.Background(), "rtok12345678")
	var le *LoginError
	if !errors.As(err, &le) || le.Code != CodeSecretStore || le.Message != msgSecretStore {
		t.Fatalf("err %v", err)
	}
	if st := e.sess.State(); st.State != StateLoggedOut || st.ErrorCode != CodeSecretStore {
		t.Fatalf("state %+v", st)
	}
	e.auth.mu.Lock()
	inv := append([]string(nil), e.auth.invalidated...)
	e.auth.mu.Unlock()
	if len(inv) != 1 || inv[0] != "acc-live-777" || e.tokens.get() != "" {
		t.Fatalf("invalidated %v token %q", inv, e.tokens.get())
	}
	if strings.Contains(e.logs.String(), "acc-live-777") {
		t.Fatal("token leaked into logs")
	}
}
