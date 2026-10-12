// internal/kite/logout_test.go
package kite

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
)

// FR-ACCT-004 AC1.
func TestLogout_InvalidatesDeletesAndStops(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	st := loginViaPaste(t, e)
	res, err := e.sess.Logout(context.Background())
	if err != nil || !res.RemoteInvalidated {
		t.Fatalf("res %+v err %v", res, err)
	}
	if len(e.auth.invalidated) != 1 || e.auth.invalidated[0] != "acc-live-777" {
		t.Fatalf("invalidated %v", e.auth.invalidated)
	}
	if e.secrets.has(secrets.KeyAccessToken) || e.feed.stopCount() != 1 || !e.lim.isPaused() || e.tokens.get() != "" {
		t.Fatal("token kept, ticker running or limiter not paused")
	}
	if got := e.sess.State(); got.State != StateLoggedOut {
		t.Fatalf("state %+v", got)
	}
	if row, _ := e.repo.Get(context.Background(), st.SessionID); row.EndReason != "logout" {
		t.Fatalf("row %+v", row)
	}
	if !e.secrets.has(secrets.KeyAPISecret) {
		t.Fatal("logout must keep the credentials")
	}
	if strings.Contains(e.logs.String(), "acc-live-777") {
		t.Fatal("token leaked into logs")
	}
}

func TestLogout_OfflineStillLogsOutLocally(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	loginViaPaste(t, e)
	e.auth.invErr = APIError("NetworkException", 503, "Request failed.")
	res, err := e.sess.Logout(context.Background())
	if err != nil || res.RemoteInvalidated || res.Message == "" {
		t.Fatalf("res %+v err %v", res, err)
	}
	if e.sess.State().State != StateLoggedOut || e.secrets.has(secrets.KeyAccessToken) {
		t.Fatal("local logout incomplete")
	}
}

// FR-ACCT-005 AC3.
func TestRemoveCredentials(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	loginViaPaste(t, e)
	st, err := e.sess.RemoveCredentials(context.Background())
	if err != nil || st.State != StateNoCredentials {
		t.Fatalf("st %+v err %v", st, err)
	}
	if e.secrets.has(secrets.KeyAPISecret) || e.secrets.has(secrets.KeyAccessToken) || settings.Get(e.settings, SettingAPIKey) != "" {
		t.Fatal("key, secret or token survived")
	}
}

func TestSetRedirectPort(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	info, err := e.sess.SetRedirectPort(8123)
	if err != nil || info.RedirectURL != "http://127.0.0.1:8123/kite/callback" {
		t.Fatalf("info %+v err %v", info, err)
	}
	var le *LoginError
	if _, err := e.sess.SetRedirectPort(80); !errors.As(err, &le) || le.Code != CodeInvalidPort {
		t.Fatalf("port 80 err %v", err)
	}
	setPort(t, e, freePort(t))
	if _, err := e.sess.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.sess.SetRedirectPort(8124); !errors.As(err, &le) || le.Code != CodeLoginBusy {
		t.Fatalf("during login err %v", err)
	}
}

// FR-ACCT-004 AC1 on the wire: DELETE /session/token with api_key and access_token.
func TestKcAuth_InvalidateSendsDelete(t *testing.T) {
	var method, path, key, tok string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		key, tok = r.URL.Query().Get("api_key"), r.URL.Query().Get("access_token")
		io.WriteString(w, `{"status":"success","data":true}`)
	}))
	defer srv.Close()
	if err := NewAuth(srv.Client(), srv.URL)(testKey).Invalidate(context.Background(), "acc-1"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete || path != "/session/token" || key != testKey || tok != "acc-1" {
		t.Fatalf("%s %s key=%s tok=%s", method, path, key, tok)
	}
}

// Ruling 57: SetRedirectPort's guidance tells the user to change the redirect URL of the Kite developer
// app to RedirectURL(port); a successful change logs the same guidance.
func TestSetRedirectPort_GuidanceNamesRedirectURL(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	var le *LoginError
	if _, err := e.sess.SetRedirectPort(80); !errors.As(err, &le) || !strings.Contains(le.Message, "1024") || !strings.Contains(le.Message, "redirect URL") {
		t.Fatalf("invalid port guidance: %v", err)
	}
	if _, err := e.sess.SetRedirectPort(8123); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.logs.String(), RedirectURL(8123)) {
		t.Fatalf("no guidance logged: %s", e.logs.String())
	}
	setPort(t, e, freePort(t))
	if _, err := e.sess.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.sess.SetRedirectPort(8124); !errors.As(err, &le) || !strings.Contains(le.Message, RedirectURL(8124)) || !strings.Contains(le.Message, "Kite developer") {
		t.Fatalf("busy guidance: %v", err)
	}
}

// Carry-forward (Task 6): a token Delete that times out behind a hung Set of the same key is reported,
// never claimed as success; the local logout still completes and a retry deletes the token.
func TestLogout_TokenDeleteTimesOutBehindHungSet(t *testing.T) {
	shortSecretsTimeout(t)
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	loginViaPaste(t, e)
	g := &gatedSecrets{memSecrets: e.secrets, gate: make(chan struct{})}
	e.sess.d.Secrets = newBoundedSecrets(g)
	// A Set of the token key hangs (e.g. a keyring waiting on an unlock prompt).
	if err := e.sess.d.Secrets.Set(secrets.KeyAccessToken, "hung-token-value"); !errors.Is(err, secrets.ErrUnavailable) {
		t.Fatalf("set: %v", err)
	}
	res, err := e.sess.Logout(context.Background())
	var le *LoginError
	if !errors.As(err, &le) || le.Code != CodeSecretStore || le.Message != msgSecretStore {
		t.Fatalf("err %v", err)
	}
	if !res.RemoteInvalidated || res.Message == "" {
		t.Fatalf("res %+v", res)
	}
	st := e.sess.State()
	if st.State != StateLoggedOut || st.ErrorCode != CodeSecretStore {
		t.Fatalf("state %+v", st)
	}
	if e.feed.stopCount() != 1 || !e.lim.isPaused() || e.tokens.get() != "" {
		t.Fatal("local logout incomplete")
	}
	close(g.gate)
	eventually(t, "hung set landed", func() bool { return e.secrets.has(secrets.KeyAccessToken) })
	if _, err := e.sess.Logout(context.Background()); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if e.secrets.has(secrets.KeyAccessToken) {
		t.Fatal("token survived the retried logout")
	}
	if st := e.sess.State(); st.State != StateLoggedOut || st.ErrorCode != "" {
		t.Fatalf("state after retry %+v", st)
	}
	if strings.Contains(e.logs.String(), "acc-live-777") || strings.Contains(e.logs.String(), "hung-token-value") {
		t.Fatal("token leaked into logs")
	}
}

// Carry-forward (Task 8): the end-of-session side effects run once. Logging out after the session already
// expired does not stop the feed again, does not call Kite and keeps the row's end reason.
func TestLogout_AfterExpiry_SideEffectsOnce(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	st := loginViaPaste(t, e)
	e.sess.HandleTokenError(APIError(kiteconnect.TokenError, 403, "Incorrect `api_key` or `access_token`."))
	assertExpired(t, e, CodeTokenRejected)
	res, err := e.sess.Logout(context.Background())
	if err != nil || res.RemoteInvalidated {
		t.Fatalf("res %+v err %v", res, err)
	}
	if e.feed.stopCount() != 1 || len(e.auth.invalidated) != 0 {
		t.Fatalf("stops %d invalidated %v", e.feed.stopCount(), e.auth.invalidated)
	}
	if row, _ := e.repo.Get(context.Background(), st.SessionID); row.EndReason != CodeTokenRejected {
		t.Fatalf("row %+v", row)
	}
	if got := e.sess.State(); got.State != StateLoggedOut {
		t.Fatalf("state %+v", got)
	}
}

// Logout closes a pending login: the listener and the attempt's nonce are retired.
func TestLogout_DuringLogin_ClosesListener(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.sess.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "listener closed", func() bool {
		resp, err := http.Get(ls.RedirectURL)
		if err == nil {
			resp.Body.Close()
		}
		return err != nil
	})
	if st := e.sess.State(); st.State != StateLoggedOut {
		t.Fatalf("state %+v", st)
	}
}
