package kite

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

// Ruling 37: AckRisk must not report success when nothing was persisted.
func TestAckRisk_NotPersisted_ReportsErrorAndKeepsAckRequired(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 14, 0))
	e.configure()
	e.seedToken(ist(2026, 10, 12, 9, 0), false)
	if st := e.sess.Restore(context.Background()); !st.RiskAckRequired {
		t.Fatalf("ack should be required: %+v", st)
	}
	if err := e.db.Close(); err != nil {
		t.Fatal(err)
	}
	st, err := e.sess.AckRisk(context.Background())
	if err == nil {
		t.Fatal("AckRisk reported success although app.db could not record it")
	}
	if !st.RiskAckRequired || !e.sess.State().RiskAckRequired {
		t.Fatalf("ack must still be required: %+v", st)
	}
}

// Ruling 37: a token whose sessions row is missing (ErrNoRow) gets its row back, so the ack persists.
func TestAckRisk_MissingRow_IsRecreatedAndPersisted(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 14, 0))
	e.configure()
	rec := e.seedToken(ist(2026, 10, 12, 9, 0), false)
	// Simulate an app.db without the row: point the token at a session id the repo has never seen.
	rec.SessionID = "sess-orphan"
	b := `{"accessToken":"` + rec.AccessToken + `","userId":"AB1234","sessionId":"sess-orphan","loggedInAtMs":` +
		itoa(rec.LoggedInAtMs) + `,"expiresAtMs":` + itoa(rec.ExpiresAtMs) + `}`
	if err := e.secrets.Set(secrets.KeyAccessToken, b); err != nil {
		t.Fatal(err)
	}
	if _, err := e.repo.Get(context.Background(), "sess-orphan"); !errors.Is(err, store.ErrNoRow) {
		t.Fatalf("precondition: %v", err)
	}
	if st := e.sess.Restore(context.Background()); st.State != StateReady || !st.RiskAckRequired {
		t.Fatalf("state %+v", st)
	}
	// Restore recreated the row; drop it again so AckRisk itself meets ErrNoRow.
	if _, err := e.db.W.ExecContext(context.Background(), `DELETE FROM sessions WHERE id = 'sess-orphan'`); err != nil {
		t.Fatal(err)
	}
	st, err := e.sess.AckRisk(context.Background())
	if err != nil || st.RiskAckRequired {
		t.Fatalf("st %+v err %v", st, err)
	}
	if l := e.logs.String(); !strings.Contains(l, `"level":"WARN","msg":"risk acknowledgement: session row missing; recreating it","session_id":"sess-orphan"`) {
		t.Fatalf("recreating the row must log a Warn with session_id: %s", l)
	}
	row, err := e.repo.Get(context.Background(), "sess-orphan")
	if err != nil || row.RiskAckAtMs != ist(2026, 10, 12, 14, 0).UnixMilli() || row.UserID != "AB1234" {
		t.Fatalf("row %+v err %v", row, err)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// hangingSecrets never answers until released (a locked keyring collection waiting on a prompt).
type hangingSecrets struct {
	release chan struct{}
	calls   *atomic.Int32
}

func (h hangingSecrets) Get(secrets.Key) (string, error) {
	h.calls.Add(1)
	<-h.release
	return "", secrets.ErrNotFound
}
func (h hangingSecrets) Set(secrets.Key, string) error { h.calls.Add(1); <-h.release; return nil }
func (h hangingSecrets) Delete(secrets.Key) error      { h.calls.Add(1); <-h.release; return nil }
func (h hangingSecrets) Backend() string               { return "hanging" }

func shortSecretsTimeout(t *testing.T) {
	old := secretsTimeout
	secretsTimeout = 50 * time.Millisecond
	t.Cleanup(func() { secretsTimeout = old })
}

// Carry-forward (Task 6/27) and Ruling 52 (1), (3), (6): a secret-store call never hangs the UI path, an
// unavailable store never reads as "no credentials", and a hung key is not called again.
func TestSession_SecretStoreCallsAreTimeBounded(t *testing.T) {
	shortSecretsTimeout(t)
	e := newEnv(t, ist(2026, 10, 12, 9, 0))
	if err := settings.Set(context.Background(), e.settings, SettingAPIKey, testKey); err != nil {
		t.Fatal(err)
	}
	h := hangingSecrets{release: make(chan struct{}), calls: &atomic.Int32{}}
	t.Cleanup(func() { close(h.release) })
	sess := NewSession(SessionDeps{Settings: e.settings, Secrets: h, Repo: e.repo, Clock: e.clock, Bus: e.bus,
		Auth: func(string) AuthAPI { return e.auth }, Profiles: e.profiles, Tokens: e.tokens, Limiter: e.lim, Feed: e.feed, Probe: e.probe,
		Log: slog.New(slog.NewJSONHandler(e.logs, nil))})

	done := make(chan struct{})
	go func() {
		defer close(done)
		st := sess.Restore(context.Background())
		if st.State != StateLoggedOut || st.ErrorCode != CodeSecretStore || st.ErrorMessage != msgSecretStore {
			t.Errorf("restore state %+v", st)
		}
		if !e.lim.isPaused() {
			t.Error("limiter must stay paused without a readable session")
		}
		_, err := sess.SaveCredentials(context.Background(), testKey, testSecret)
		var le *LoginError
		if !errors.As(err, &le) || le.Code != CodeSecretStore || le.Message != msgSecretStore {
			t.Errorf("save err %#v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a hanging secret store blocked the session")
	}
	if n := h.calls.Load(); n != 1 {
		t.Fatalf("the hung key was called %d times; want 1 (single-flight)", n)
	}
	if strings.Contains(e.logs.String(), testSecret) {
		t.Fatal("secret found in logs")
	}
	if !strings.Contains(e.logs.String(), "did not answer") {
		t.Fatalf("the cause must be logged: %s", e.logs.String())
	}
}

// gatedSecrets blocks Set until the gate closes; everything else answers at once.
type gatedSecrets struct {
	*memSecrets
	gate    chan struct{}
	deletes atomic.Int32
}

func (g *gatedSecrets) Set(k secrets.Key, v string) error { <-g.gate; return g.memSecrets.Set(k, v) }
func (g *gatedSecrets) Delete(k secrets.Key) error {
	g.deletes.Add(1)
	return g.memSecrets.Delete(k)
}

// Ruling 52 (3): within() is single-flight per key — no goroutine pile-up while a key hangs, and a
// timed-out Set can never land after a later Delete.
func TestBoundedSecrets_SingleFlightPerKey(t *testing.T) {
	shortSecretsTimeout(t)
	g := &gatedSecrets{memSecrets: &memSecrets{m: map[secrets.Key]string{}}, gate: make(chan struct{})}
	b := newBoundedSecrets(g)
	if err := b.Set(secrets.KeyAccessToken, "late-token-value"); !errors.Is(err, secrets.ErrUnavailable) {
		t.Fatalf("set: %v", err)
	}
	if err := b.Delete(secrets.KeyAccessToken); !errors.Is(err, secrets.ErrUnavailable) {
		t.Fatalf("delete while the set is in flight: %v", err)
	}
	if g.deletes.Load() != 0 {
		t.Fatal("Delete reached the store while a Set of the same key was still in flight")
	}
	// Other keys are not blocked by the hung one.
	if _, err := b.Get(secrets.KeyAPISecret); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("other key: %v", err)
	}
	close(g.gate)
	eventually(t, "late set landed", func() bool { return g.has(secrets.KeyAccessToken) })
	if err := b.Delete(secrets.KeyAccessToken); err != nil {
		t.Fatal(err)
	}
	if g.has(secrets.KeyAccessToken) {
		t.Fatal("token survived the delete")
	}

	h := hangingSecrets{release: make(chan struct{}), calls: &atomic.Int32{}}
	defer close(h.release)
	hb := newBoundedSecrets(h)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = hb.Get(secrets.KeyAPISecret) }()
	}
	wg.Wait()
	if n := h.calls.Load(); n != 1 {
		t.Fatalf("%d store calls for one hung key; want 1", n)
	}
}

// Ruling 52 (2): the feed can never start with a token that endSession just deleted.
func TestFeedStart_SerialisedWithEndSession(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 14, 0))
	e.configure()
	rec := e.seedToken(ist(2026, 10, 12, 9, 0), true)
	gate := make(chan struct{})
	e.feed.mu.Lock()
	e.feed.startGate = gate
	e.feed.mu.Unlock()
	e.sess.Restore(context.Background())
	eventually(t, "feed start entered", func() bool { return e.feed.enteredCount() == 1 })

	ended := make(chan struct{})
	go func() { defer close(ended); e.sess.endSession(rec, CodeSessionExpired) }()
	time.Sleep(50 * time.Millisecond)
	if e.feed.stopCount() != 0 {
		t.Fatal("endSession stopped the feed while Feed.Start was still running")
	}
	close(gate)
	<-ended
	if got := e.feed.eventLog(); len(got) != 2 || got[0] != "start" || got[1] != "stop" {
		t.Fatalf("feed events %v; the last must be stop", got)
	}
}

// Ruling 52 (2): a probe that finishes after the session ended never starts the feed.
func TestProbe_AfterEndSession_DoesNotStartFeed(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 14, 0))
	e.configure()
	rec := e.seedToken(ist(2026, 10, 12, 9, 0), true)
	gate := make(chan struct{})
	e.probe.mu.Lock()
	e.probe.gate = gate
	e.probe.mu.Unlock()
	e.sess.Restore(context.Background())
	eventually(t, "probe running", func() bool { return e.probe.callCount() == 1 })
	e.sess.endSession(rec, CodeSessionExpired)
	close(gate)
	time.Sleep(50 * time.Millisecond)
	if e.feed.startCount() != 0 || e.feed.enteredCount() != 0 {
		t.Fatal("feed started after the session ended")
	}
	if st := e.sess.State(); st.State != StateExpired || st.DataPlan != PlanUnknown {
		t.Fatalf("state %+v", st)
	}
}

// The OS keyring backend (go-keyring mock, never the real login Keychain) round-trips through the bound.
func TestSaveCredentials_KeyringBackend(t *testing.T) {
	keyring.MockInit()
	e := newEnv(t, ist(2026, 10, 12, 9, 0))
	sess := NewSession(SessionDeps{Settings: e.settings, Secrets: secrets.NewKeyring(), Repo: e.repo, Clock: e.clock, Bus: e.bus,
		Auth: func(string) AuthAPI { return e.auth }, Profiles: e.profiles, Tokens: e.tokens, Limiter: e.lim, Feed: e.feed, Probe: e.probe})
	st, err := sess.SaveCredentials(context.Background(), testKey, testSecret)
	if err != nil || st.State != StateLoggedOut {
		t.Fatalf("st %+v err %v", st, err)
	}
	if v, err := secrets.NewKeyring().Get(secrets.KeyAPISecret); err != nil || v != testSecret {
		t.Fatalf("keyring has %q err %v", v, err)
	}
	if info := sess.CredentialsInfo(); !info.Configured || info.SecretBackend != secrets.BackendKeyring {
		t.Fatalf("info %+v", info)
	}
}
