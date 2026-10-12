package kite

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"tradeloom/internal/secrets"
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
	st, err := e.sess.AckRisk(context.Background())
	if err != nil || st.RiskAckRequired {
		t.Fatalf("st %+v err %v", st, err)
	}
	row, err := e.repo.Get(context.Background(), "sess-orphan")
	if err != nil || row.RiskAckAtMs != ist(2026, 10, 12, 14, 0).UnixMilli() || row.UserID != "AB1234" {
		t.Fatalf("row %+v err %v", row, err)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// hangingSecrets never answers until released (a locked keyring collection waiting on a prompt).
type hangingSecrets struct{ release chan struct{} }

func (h hangingSecrets) Get(secrets.Key) (string, error) { <-h.release; return "", secrets.ErrNotFound }
func (h hangingSecrets) Set(secrets.Key, string) error   { <-h.release; return nil }
func (h hangingSecrets) Delete(secrets.Key) error        { <-h.release; return nil }
func (h hangingSecrets) Backend() string                 { return "hanging" }

// Carry-forward (Task 6/27): a secret-store call can never hang the UI path.
func TestSession_SecretStoreCallsAreTimeBounded(t *testing.T) {
	old := secretsTimeout
	secretsTimeout = 50 * time.Millisecond
	t.Cleanup(func() { secretsTimeout = old })
	e := newEnv(t, ist(2026, 10, 12, 9, 0))
	h := hangingSecrets{release: make(chan struct{})}
	t.Cleanup(func() { close(h.release) })
	sess := NewSession(SessionDeps{Settings: e.settings, Secrets: h, Repo: e.repo, Clock: e.clock, Bus: e.bus,
		Auth: func(string) AuthAPI { return e.auth }, Profiles: e.profiles, Tokens: e.tokens, Limiter: e.lim, Feed: e.feed, Probe: e.probe})

	done := make(chan struct{})
	go func() {
		defer close(done)
		if st := sess.Restore(context.Background()); st.State != StateNoCredentials {
			t.Errorf("restore state %+v", st)
		}
		_, err := sess.SaveCredentials(context.Background(), testKey, testSecret)
		var le *LoginError
		if !errors.As(err, &le) || le.Code != CodeSecretStore {
			t.Errorf("save err %v", err)
		}
		if info := sess.CredentialsInfo(); info.Configured {
			t.Errorf("info %+v", info)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a hanging secret store blocked the session")
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
