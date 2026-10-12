> **P0-3 Session & Service Layer Implementation Plan — Task 10.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 10: Logout, remove credentials, change redirect port

**Files:**
- Create: `internal/kite/logout.go`
- Test: `internal/kite/logout_test.go`

**Interfaces:**
- Consumes: Tasks 6–8.
- Produces:
  ```go
  type LogoutResult struct{ RemoteInvalidated bool; Message string } // TS kite.LogoutResult
  const CodeInvalidPort = "invalid_port"
  func (s *Session) Logout(ctx context.Context) (LogoutResult, error)
  func (s *Session) RemoveCredentials(ctx context.Context) (SessionState, error)
  func (s *Session) SetRedirectPort(port int) (CredentialsInfo, error)
  ```

Logout order: close the listener → stop the ticker → pause the limiter → `DELETE /session/token` (best effort; local logout always completes) → end the session row → delete the token secret → publish `logged_out`. FR-ACCT-004's "pause deployments and scanners" is P3/P4. Those engines subscribe to `session.state` when they ship.

- [ ] **Step 1: Write the failing tests**

```go
// internal/kite/logout_test.go
package kite

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestLogout|TestRemoveCredentials|TestSetRedirectPort|TestKcAuth_Invalidate' -v`
Expected: FAIL to compile with `undefined: (*Session).Logout`.

- [ ] **Step 3: Write the implementation**

```go
// internal/kite/logout.go
package kite

import (
	"context"
	"errors"

	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
)

const CodeInvalidPort = "invalid_port"

type LogoutResult struct {
	RemoteInvalidated bool   `json:"remoteInvalidated"`
	Message           string `json:"message"`
}

// Logout implements FR-ACCT-004: invalidate at Kite, delete the token, stop the ticker, pause all Kite traffic.
func (s *Session) Logout(ctx context.Context) (LogoutResult, error) {
	s.exMu.Lock()
	defer s.exMu.Unlock()

	s.mu.Lock()
	rec := s.rec
	s.rec = nil
	lb := s.lb
	s.lb = nil
	s.gen++
	s.done = map[string]bool{}
	s.mu.Unlock()

	if lb != nil {
		lb.Close()
	}
	s.d.Feed.Stop()
	s.d.Limiter.PauseAll("logged out")
	res := LogoutResult{}
	if rec != nil {
		if err := s.d.Auth(s.apiKey()).Invalidate(ctx, rec.AccessToken); err != nil {
			res.Message = "Logged out of Tradeloom. Kite could not be reached to invalidate the token; it stops working at 06:00 IST."
			s.d.Log.Warn("invalidating the access token failed", "kind", errKind(err))
		} else {
			res.RemoteInvalidated = true
		}
		s.endRow(rec.SessionID, "logout")
	}
	s.d.Tokens.SetAccessToken("")
	if err := s.d.Secrets.Delete(secrets.KeyAccessToken); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return res, &LoginError{Code: CodeSecretStore, Message: "The access token could not be removed from the secret store: " + err.Error()}
	}
	s.set(s.idle("", ""))
	s.d.Log.Info("kite logout", "remote_invalidated", res.RemoteInvalidated)
	return res, nil
}

// RemoveCredentials implements FR-ACCT-005 "Remove credentials": logout, then delete the key, secret and token.
func (s *Session) RemoveCredentials(ctx context.Context) (SessionState, error) {
	if s.State().State == StateLoggingIn {
		s.CancelLogin()
	}
	if _, err := s.Logout(ctx); err != nil {
		return s.State(), err
	}
	if err := s.d.Secrets.Delete(secrets.KeyAPISecret); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return s.State(), &LoginError{Code: CodeSecretStore, Message: "The API secret could not be removed from the secret store: " + err.Error()}
	}
	if err := settings.Set(ctx, s.d.Settings, SettingAPIKey, ""); err != nil {
		return s.State(), err
	}
	s.d.Tokens.SetAPIKey("")
	s.d.Log.Info("kite credentials removed")
	return s.set(SessionState{State: StateNoCredentials, DataPlan: PlanUnknown}), nil
}

// SetRedirectPort changes the loopback port; the user must register the new RedirectURL in the developer console.
func (s *Session) SetRedirectPort(port int) (CredentialsInfo, error) {
	if s.State().State == StateLoggingIn {
		return s.CredentialsInfo(), &LoginError{Code: CodeLoginBusy, Message: "Finish or cancel the current login before changing the port."}
	}
	if err := settings.Set(context.Background(), s.d.Settings, SettingRedirectPort, port); err != nil {
		return s.CredentialsInfo(), &LoginError{Code: CodeInvalidPort, Message: "Choose a port between 1024 and 65535."}
	}
	return s.CredentialsInfo(), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/kite/ -run 'TestLogout|TestRemoveCredentials|TestSetRedirectPort|TestKcAuth_Invalidate' -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/kite/logout.go internal/kite/logout_test.go
git commit -m "feat(kite): logout, credential removal and redirect port change

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
