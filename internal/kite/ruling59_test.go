// internal/kite/ruling59_test.go
package kite

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/bus"
	"tradeloom/internal/secrets"
)

// gatedProfiles blocks Profile until gate closes; entered is closed when the first call begins.
type gatedProfiles struct {
	*fakeProfiles
	gate    chan struct{}
	entered chan struct{}
}

func (g *gatedProfiles) Profile(ctx context.Context) (kiteconnect.UserProfile, error) {
	select {
	case <-g.entered:
	default:
		close(g.entered)
	}
	<-g.gate
	return g.fakeProfiles.Profile(ctx)
}

// gatedAuth wraps fakeAuth; GenerateSession and Invalidate block on their gates when set.
type gatedAuth struct {
	*fakeAuth
	genGate, invGate       chan struct{}
	genEntered, invEntered chan struct{}
}

func (g *gatedAuth) GenerateSession(ctx context.Context, rt, sec string) (AuthSession, error) {
	if g.genGate != nil {
		close(g.genEntered)
		<-g.genGate
	}
	return g.fakeAuth.GenerateSession(ctx, rt, sec)
}

func (g *gatedAuth) Invalidate(ctx context.Context, tok string) error {
	if g.invGate != nil {
		close(g.invEntered)
		<-g.invGate
	}
	return g.fakeAuth.Invalidate(ctx, tok)
}

// waitBriefly polls cond for up to 200 ms: long enough for unserialised code to get there first (the
// bug), and harmlessly expiring when the code under test correctly keeps the other party waiting.
func waitBriefly(cond func() bool) {
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) && !cond() {
		time.Sleep(2 * time.Millisecond)
	}
}

func feedIdle(e *testEnv) bool {
	ev := e.feed.eventLog()
	return len(ev) == 0 || ev[len(ev)-1] == "stop"
}

// Ruling 59 (1): a Logout racing Restore's Profile call wins: the restored token is invalidated and
// deleted, the session ends logged out and no feed is left running.
func TestRestore_LogoutDuringProfile_LogoutWins(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	rec := e.seedToken(ist(2026, 10, 12, 8, 0), true)
	gp := &gatedProfiles{fakeProfiles: e.profiles, gate: make(chan struct{}), entered: make(chan struct{})}
	e.sess.d.Profiles = gp
	restored := make(chan struct{})
	go func() { defer close(restored); e.sess.Restore(context.Background()) }()
	<-gp.entered
	loggedOut := make(chan struct{})
	go func() {
		defer close(loggedOut)
		if _, err := e.sess.Logout(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	waitBriefly(func() bool {
		select {
		case <-loggedOut:
			return true
		default:
			return false
		}
	})
	close(gp.gate)
	<-restored
	<-loggedOut

	if st := e.sess.State(); st.State != StateLoggedOut {
		t.Fatalf("state %+v", st)
	}
	e.auth.mu.Lock()
	inv := append([]string(nil), e.auth.invalidated...)
	e.auth.mu.Unlock()
	if len(inv) != 1 || inv[0] != rec.AccessToken {
		t.Fatalf("invalidated %v", inv)
	}
	if e.secrets.has(secrets.KeyAccessToken) || e.tokens.get() != "" || !e.lim.isPaused() {
		t.Fatal("token kept or limiter running")
	}
	// A probe started by Restore may finish after the logout; it must not leave a feed running.
	waitBriefly(func() bool { return !feedIdle(e) })
	if !feedIdle(e) {
		t.Fatalf("feed left running: %v", e.feed.eventLog())
	}
}

// Ruling 59 (1): a paste login committing while Restore waits on Kite survives Restore.
func TestRestore_PasteLoginDuringProfile_NewSessionSurvives(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	e.seedToken(ist(2026, 10, 12, 8, 0), true)
	gp := &gatedProfiles{fakeProfiles: e.profiles, gate: make(chan struct{}), entered: make(chan struct{})}
	e.sess.d.Profiles = gp
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	restored := make(chan struct{})
	go func() { defer close(restored); e.sess.Restore(context.Background()) }()
	<-gp.entered
	if _, err := e.sess.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	logged := make(chan struct{})
	go func() {
		defer close(logged)
		if _, err := e.sess.CompleteLogin(context.Background(), "rtok12345678"); err != nil {
			t.Error(err)
		}
	}()
	waitBriefly(func() bool { return e.sess.State().State == StateReady })
	close(gp.gate)
	<-restored
	<-logged

	st := e.sess.State()
	if st.State != StateReady || st.SessionID != "sess-1" {
		t.Fatalf("new session lost: %+v", st)
	}
	raw, _ := e.secrets.Get(secrets.KeyAccessToken)
	if !strings.Contains(raw, "acc-live-777") || e.tokens.get() != "acc-live-777" {
		t.Fatalf("stored %q in-memory %q", raw, e.tokens.get())
	}
	eventually(t, "feed started", func() bool { return e.feed.startCount() >= 1 })
	e.feed.mu.Lock()
	tok := e.feed.token
	e.feed.mu.Unlock()
	if tok != "acc-live-777" {
		t.Fatalf("feed started with %q", tok)
	}
}

// failDeleteSecrets refuses to delete the access token (a store that keeps failing).
type failDeleteSecrets struct{ *memSecrets }

func (f failDeleteSecrets) Delete(k secrets.Key) error {
	if k == secrets.KeyAccessToken {
		return fmt.Errorf("%w: keyring locked", secrets.ErrUnavailable)
	}
	return f.memSecrets.Delete(k)
}

// Ruling 59 (2): a logout that could neither invalidate nor delete the token never logs back in after
// a restart: the stored token belongs to an ended session row, so Restore deletes it and stays idle.
func TestRestore_EndedSessionRow_NoAutoRelogin(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	st := loginViaPaste(t, e)
	e.auth.invErr = APIError("NetworkException", 503, "Request failed.")
	e.sess.d.Secrets = newBoundedSecrets(failDeleteSecrets{e.secrets})
	if _, err := e.sess.Logout(context.Background()); err == nil {
		t.Fatal("logout must report the failed delete")
	}
	if !e.secrets.has(secrets.KeyAccessToken) {
		t.Fatal("precondition: the token is still stored")
	}
	if row, _ := e.repo.Get(context.Background(), st.SessionID); row.EndedAtMs == 0 {
		t.Fatalf("precondition: row not ended %+v", row)
	}

	// Restart: a new Session over the same stores.
	restarted := NewSession(SessionDeps{Settings: e.settings, Secrets: e.secrets, Repo: e.repo, Clock: e.clock, Bus: e.bus,
		Auth: func(string) AuthAPI { return e.auth }, Profiles: e.profiles, Tokens: e.tokens, Limiter: e.lim, Feed: e.feed, Probe: e.probe,
		Log: slog.New(slog.NewJSONHandler(e.logs, nil))})
	calls := e.profiles.callCount()
	got := restarted.Restore(context.Background())
	if got.State != StateLoggedOut {
		t.Fatalf("restored %+v", got)
	}
	if e.profiles.callCount() != calls {
		t.Fatal("Restore called Kite for an ended session")
	}
	if e.secrets.has(secrets.KeyAccessToken) {
		t.Fatal("the ended session's token was not deleted")
	}
	if !e.lim.isPaused() || e.tokens.get() != "" {
		t.Fatal("limiter running or token applied")
	}
}

// Ruling 59 (3): a nonce is bound to its exchanged token only when its attempt is still the current one
// at commit time; a nonce retired during the exchange stays retired.
func TestLogin_NonceRetiredDuringExchange_NotRebound(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	ga := &gatedAuth{fakeAuth: e.auth, genGate: make(chan struct{}), genEntered: make(chan struct{})}
	e.sess.d.Auth = func(string) AuthAPI { return ga }
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	u := ls.RedirectURL + "?request_token=rtok12345678&status=success&state=" + stateFrom(t, ls.LoginURL)
	first := make(chan int, 1)
	go func() {
		resp, err := http.Get(u)
		if err != nil {
			first <- 0
			return
		}
		resp.Body.Close()
		first <- resp.StatusCode
	}()
	<-ga.genEntered
	// A restart retires the nonce while its exchange is in flight (the listener stays open).
	if _, err := e.sess.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(ga.genGate)
	if code := <-first; code != http.StatusOK {
		t.Fatalf("in-flight callback: %d", code)
	}
	if st := e.sess.State(); st.State != StateReady {
		t.Fatalf("state %+v", st)
	}
	e.sess.mu.Lock()
	bound := e.sess.nonceAcceptsLocked(stateFrom(t, ls.LoginURL), "rtok12345678")
	e.sess.mu.Unlock()
	if bound {
		t.Fatal("a retired nonce was re-bound after the exchange")
	}
	if code, _ := httpGet(t, u); code != http.StatusBadRequest {
		t.Fatalf("reload with a retired nonce: %d", code)
	}
}

// Ruling 59 (4): Logout publishes the interim logged_out state before it calls Kite.
func TestLogout_PublishesInterimStateBeforeKiteCall(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	loginViaPaste(t, e)
	ga := &gatedAuth{fakeAuth: e.auth, invGate: make(chan struct{}), invEntered: make(chan struct{})}
	e.sess.d.Auth = func(string) AuthAPI { return ga }
	sub := bus.Subscribe(e.bus, TopicSessionState, bus.SubOpts[SessionState]{Name: "test.logout", Buffer: 16})
	defer bus.Unsubscribe(e.bus, sub)
	done := make(chan struct{})
	go func() { defer close(done); _, _ = e.sess.Logout(context.Background()) }()
	<-ga.invEntered
	timeout := time.After(2 * time.Second)
	for got := false; !got; {
		select {
		case st := <-sub.C():
			got = st.State == StateLoggedOut
		case <-timeout:
			close(ga.invGate)
			<-done
			t.Fatal("no logged_out published before the Kite call")
		}
	}
	close(ga.invGate)
	<-done
}
