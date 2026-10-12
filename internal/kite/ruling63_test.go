// internal/kite/ruling63_test.go
package kite

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/secrets"
)

// restarted returns a fresh Session over e's stores, as after an app restart (initial state no_credentials).
func restarted(t *testing.T, e *testEnv) *Session {
	s := NewSession(SessionDeps{Settings: e.settings, Secrets: e.secrets, Repo: e.repo, Clock: e.clock, Bus: e.bus,
		Auth: func(string) AuthAPI { return e.auth }, Profiles: e.profiles, Tokens: e.tokens, Limiter: e.lim, Feed: e.feed, Probe: e.probe,
		Log: slog.New(slog.NewJSONHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))})
	t.Cleanup(s.Close)
	return s
}

// endSeeded seeds a stored token whose session row ended with reason.
func endSeeded(t *testing.T, e *testEnv, loginAt time.Time, reason string) TokenRecord {
	t.Helper()
	rec := e.seedToken(loginAt, true)
	if err := e.repo.End(context.Background(), rec.SessionID, loginAt.UnixMilli()+60_000, reason); err != nil {
		t.Fatal(err)
	}
	return rec
}

func (f *fakeAuth) invalidatedTokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.invalidated...)
}

// Ruling 63 (1): a login begun and cancelled while Restore waited on Kite leaves the state no_credentials
// (BeginLogin's fallback from the initial state). Credentials exist, so the abandon branch corrects it to
// logged_out and publishes the correction.
func TestRestore_Abandoned_CorrectsNoCredentialsToLoggedOut(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	e.seedToken(ist(2026, 10, 12, 8, 0), true)
	setPort(t, e, freePort(t))
	s := restarted(t, e)
	gp := &gatedProfiles{fakeProfiles: e.profiles, gate: make(chan struct{}), entered: make(chan struct{})}
	s.d.Profiles = gp
	restored := make(chan SessionState, 1)
	go func() { restored <- s.Restore(context.Background()) }()
	<-gp.entered
	if _, err := s.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := s.CancelLogin(); st.State != StateNoCredentials {
		t.Fatalf("precondition: cancel falls back to the initial state, got %+v", st)
	}
	sub := bus.Subscribe(e.bus, TopicSessionState, bus.SubOpts[SessionState]{Name: "test.r63", Buffer: 16})
	defer bus.Unsubscribe(e.bus, sub)
	close(gp.gate)
	got := <-restored
	if got.State != StateLoggedOut || s.State().State != StateLoggedOut {
		t.Fatalf("restore returned %+v, state %+v; want logged_out", got, s.State())
	}
	timeout := time.After(2 * time.Second)
	for published := false; !published; {
		select {
		case st := <-sub.C():
			published = st.State == StateLoggedOut
		case <-timeout:
			t.Fatal("the corrected logged_out state was not published")
		}
	}
	if e.tokens.get() != "" || !e.lim.isPaused() {
		t.Fatal("abandoned restore left the token applied or the limiter running")
	}
}

// gatedDeleteSecrets blocks Delete of the access token until gate closes.
type gatedDeleteSecrets struct {
	*memSecrets
	gate    chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (g *gatedDeleteSecrets) Delete(k secrets.Key) error {
	if k == secrets.KeyAccessToken {
		g.once.Do(func() { close(g.entered) })
		<-g.gate
	}
	return g.memSecrets.Delete(k)
}

// Ruling 63 (2): the ended-row path respects gen. A login begun while Restore deletes an ended session's
// token owns the state; Restore must not overwrite logging_in with logged_out.
func TestRestore_EndedRow_DoesNotOverwriteLoginInProgress(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	endSeeded(t, e, ist(2026, 10, 12, 8, 0), "session_expired")
	setPort(t, e, freePort(t))
	s := restarted(t, e)
	gs := &gatedDeleteSecrets{memSecrets: e.secrets, gate: make(chan struct{}), entered: make(chan struct{})}
	s.d.Secrets = newBoundedSecrets(gs)
	restored := make(chan SessionState, 1)
	go func() { restored <- s.Restore(context.Background()) }()
	<-gs.entered
	if _, err := s.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(gs.gate)
	got := <-restored
	if st := s.State(); st.State != StateLoggingIn {
		t.Fatalf("restore overwrote the login in progress: state %+v (returned %+v)", st, got)
	}
	if got.State != StateLoggingIn {
		t.Fatalf("restore returned %+v; want the current logging_in state", got)
	}
	if e.secrets.has(secrets.KeyAccessToken) {
		t.Fatal("the ended session's token was not deleted")
	}
	// The login falls back to logged_out (credentials exist), not to the initial no_credentials.
	if st := s.CancelLogin(); st.State != StateLoggedOut {
		t.Fatalf("cancel after restore: %+v", st)
	}
}

// Ruling 63 (3): a session-row read error other than ErrNoRow fails closed: no resume, no Kite call.
func TestRestore_RowReadError_FailsClosed(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	e.seedToken(ist(2026, 10, 12, 8, 0), true)
	if _, err := e.db.W.ExecContext(context.Background(), "DROP TABLE sessions"); err != nil {
		t.Fatal(err)
	}
	s := restarted(t, e)
	calls := e.profiles.callCount()
	got := s.Restore(context.Background())
	if got.State != StateLoggedOut || s.State().State != StateLoggedOut {
		t.Fatalf("restore resumed or misreported on a row-read error: %+v", got)
	}
	if e.profiles.callCount() != calls {
		t.Fatal("Restore called Kite although the session row could not be read")
	}
	if e.tokens.get() != "" || !e.lim.isPaused() {
		t.Fatal("token applied or limiter running")
	}
	if e.probe.callCount() != 0 || e.feed.startCount() != 0 {
		t.Fatal("probe or feed started")
	}
}

// Ruling 63 (4): an ended "logout" row with an unexpired token gets a best-effort Kite invalidation off
// the startup path; the outcome is logged at Warn with the error kind.
func TestRestore_EndedLogoutRow_InvalidatesUnexpiredToken(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	rec := endSeeded(t, e, ist(2026, 10, 12, 8, 0), "logout")
	e.auth.invErr = APIError("NetworkException", 503, "Request failed.")
	s := restarted(t, e)
	if got := s.Restore(context.Background()); got.State != StateLoggedOut {
		t.Fatalf("restore %+v", got)
	}
	eventually(t, "invalidation attempted", func() bool { return len(e.auth.invalidatedTokens()) == 1 })
	if inv := e.auth.invalidatedTokens(); inv[0] != rec.AccessToken {
		t.Fatalf("invalidated %v", inv)
	}
	eventually(t, "outcome logged", func() bool {
		return strings.Contains(e.logs.String(), "restore: invalidating the ended session's token")
	})
	logs := e.logs.String()
	var line string
	for _, l := range strings.Split(logs, "\n") {
		if strings.Contains(l, "restore: invalidating the ended session's token") {
			line = l
		}
	}
	if !strings.Contains(line, `"level":"WARN"`) || !strings.Contains(line, `"kind":"NetworkException"`) || !strings.Contains(line, `"invalidated":false`) {
		t.Fatalf("outcome line %s", line)
	}
	if strings.Contains(logs, rec.AccessToken) {
		t.Fatal("the access token reached the log")
	}
}

func TestRestore_EndedRow_InvalidationOnlyForUnexpiredLogout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reason  string
		loginAt time.Time
	}{
		{"expired reason", "session_expired", ist(2026, 10, 12, 8, 0)},
		{"logout but token expired", "logout", ist(2026, 10, 11, 8, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, ist(2026, 10, 12, 9, 2))
			e.configure()
			endSeeded(t, e, tc.loginAt, tc.reason)
			s := restarted(t, e)
			if got := s.Restore(context.Background()); got.State != StateLoggedOut {
				t.Fatalf("restore %+v", got)
			}
			waitBriefly(func() bool { return len(e.auth.invalidatedTokens()) > 0 })
			if inv := e.auth.invalidatedTokens(); len(inv) != 0 {
				t.Fatalf("invalidated %v", inv)
			}
		})
	}
}

// The invalidation never blocks Restore and gives up after its bound.
func TestRestore_EndedLogoutRow_InvalidationBounded(t *testing.T) {
	old := endedInvalidateTimeout
	endedInvalidateTimeout = 50 * time.Millisecond
	t.Cleanup(func() { endedInvalidateTimeout = old })
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	endSeeded(t, e, ist(2026, 10, 12, 8, 0), "logout")
	ga := &gatedAuth{fakeAuth: e.auth, invGate: make(chan struct{}), invEntered: make(chan struct{})}
	defer close(ga.invGate)
	s := restarted(t, e)
	s.d.Auth = func(string) AuthAPI { return ga }
	done := make(chan SessionState, 1)
	go func() { done <- s.Restore(context.Background()) }()
	select {
	case got := <-done:
		if got.State != StateLoggedOut {
			t.Fatalf("restore %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Restore waited for the Kite invalidation")
	}
	select {
	case <-ga.invEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("no invalidation attempted")
	}
	eventually(t, "timeout logged", func() bool {
		l := e.logs.String()
		return strings.Contains(l, "restore: invalidating the ended session's token") && strings.Contains(l, `"kind":"timeout"`)
	})
}

// gatedGetSecrets blocks Get of the access token until gate closes.
type gatedGetSecrets struct {
	*memSecrets
	gate    chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (g *gatedGetSecrets) Get(k secrets.Key) (string, error) {
	if k == secrets.KeyAccessToken {
		g.once.Do(func() { close(g.entered) })
		<-g.gate
	}
	return g.memSecrets.Get(k)
}

// Ruling 68 (2): a login begun while Restore waits on a slow token read survives every early exit of
// Restore (token missing, unreadable record, expired token), and its fallback is logged_out.
func TestRestore_LoginDuringSlowTokenGet_Survives(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed func(e *testEnv)
	}{
		{"token missing", func(*testEnv) {}},
		{"unreadable record", func(e *testEnv) { _ = e.secrets.Set(secrets.KeyAccessToken, "{not json") }},
		{"expired token", func(e *testEnv) { e.seedToken(ist(2026, 10, 11, 8, 0), true) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, ist(2026, 10, 12, 9, 2))
			e.configure()
			tc.seed(e)
			setPort(t, e, freePort(t))
			s := restarted(t, e)
			gs := &gatedGetSecrets{memSecrets: e.secrets, gate: make(chan struct{}), entered: make(chan struct{})}
			s.d.Secrets = newBoundedSecrets(gs)
			restored := make(chan SessionState, 1)
			go func() { restored <- s.Restore(context.Background()) }()
			<-gs.entered
			if _, err := s.BeginLogin(context.Background()); err != nil {
				t.Fatal(err)
			}
			close(gs.gate)
			got := <-restored
			if st := s.State(); st.State != StateLoggingIn || got.State != StateLoggingIn {
				t.Fatalf("restore overwrote the login: state %+v, returned %+v", st, got)
			}
			if tc.name != "token missing" && e.secrets.has(secrets.KeyAccessToken) {
				t.Fatal("stale token not deleted")
			}
			if st := s.CancelLogin(); st.State != StateLoggedOut {
				t.Fatalf("fallback after cancel %+v", st)
			}
		})
	}
}
