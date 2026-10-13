// internal/kite/ruling72_test.go
package kite

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/secrets"
)

// Ruling 72 (a): endSessionAtRestore checks gen and writes the expired state in one critical section. A
// login begun right after the gen check must survive: either it waits for the commit, or the commit sees
// the new gen; it is never overwritten with expired.
func TestRestore_EndAtRestore_GenCheckAndWriteAtomic(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	e.seedToken(ist(2026, 10, 11, 8, 0), true) // expired at 06:00 today
	setPort(t, e, freePort(t))
	s := restarted(t, e)
	begun := make(chan error, 1)
	s.hookEndAtRestore = func() {
		go func() { _, err := s.BeginLogin(context.Background()); begun <- err }()
		// Give BeginLogin the chance to run inside the window; with the fix it waits for mu instead.
		select {
		case err := <-begun:
			begun <- err
		case <-time.After(200 * time.Millisecond):
		}
	}
	got := s.Restore(context.Background())
	if err := <-begun; err != nil {
		t.Fatal(err)
	}
	if st := s.State(); st.State != StateLoggingIn {
		t.Fatalf("the login begun during Restore was overwritten: state %+v (restore returned %+v)", st, got)
	}
	if e.secrets.has(secrets.KeyAccessToken) {
		t.Fatal("expired token not deleted")
	}
}

// failingGetSecrets blocks the first Get of key until gate closes and then fails it with err; later
// calls (e.g. BeginLogin's credentials check) go straight to the store.
type failingGetSecrets struct {
	*memSecrets
	key     secrets.Key
	err     error
	gate    chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (g *failingGetSecrets) Get(k secrets.Key) (string, error) {
	first := false
	if k == g.key {
		g.once.Do(func() { first = true })
	}
	if first {
		close(g.entered)
		<-g.gate
		return "", g.err
	}
	return g.memSecrets.Get(k)
}

// Ruling 72 (c): a login begun while Restore is in a slow step survives Restore's exits on a configured()
// error, an access-token read error and a token rejected by Kite's Profile call.
func TestRestore_LoginDuringSlowStep_SurvivesErrorExits(t *testing.T) {
	storeErr := errors.New("keyring: collection is locked")
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, e *testEnv, s *Session) (entered, gate chan struct{})
	}{
		{"configured() error", func(t *testing.T, e *testEnv, s *Session) (chan struct{}, chan struct{}) {
			// Unwrapped, so BeginLogin's own read of the API secret is not queued behind Restore's.
			g := &failingGetSecrets{memSecrets: e.secrets, key: secrets.KeyAPISecret, err: storeErr, gate: make(chan struct{}), entered: make(chan struct{})}
			s.d.Secrets = g
			return g.entered, g.gate
		}},
		{"token read error", func(t *testing.T, e *testEnv, s *Session) (chan struct{}, chan struct{}) {
			e.seedToken(ist(2026, 10, 12, 8, 0), true)
			g := &failingGetSecrets{memSecrets: e.secrets, key: secrets.KeyAccessToken, err: storeErr, gate: make(chan struct{}), entered: make(chan struct{})}
			s.d.Secrets = newBoundedSecrets(g)
			return g.entered, g.gate
		}},
		{"token rejected by Profile", func(t *testing.T, e *testEnv, s *Session) (chan struct{}, chan struct{}) {
			e.seedToken(ist(2026, 10, 12, 8, 0), true)
			e.profiles.setErr(APIError(kiteconnect.TokenError, 403, msgTokenRejected))
			gp := &gatedProfiles{fakeProfiles: e.profiles, gate: make(chan struct{}), entered: make(chan struct{})}
			s.d.Profiles = gp
			return gp.entered, gp.gate
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, ist(2026, 10, 12, 9, 2))
			e.configure()
			setPort(t, e, freePort(t))
			s := restarted(t, e)
			entered, gate := tc.setup(t, e, s)
			restored := make(chan SessionState, 1)
			go func() { restored <- s.Restore(context.Background()) }()
			<-entered
			if _, err := s.BeginLogin(context.Background()); err != nil {
				t.Fatal(err)
			}
			close(gate)
			got := <-restored
			if st := s.State(); st.State != StateLoggingIn || got.State != StateLoggingIn {
				t.Fatalf("restore overwrote the login: state %+v, returned %+v", st, got)
			}
			if st := s.CancelLogin(); st.State != StateLoggedOut {
				t.Fatalf("fallback after cancel %+v", st)
			}
		})
	}
}
