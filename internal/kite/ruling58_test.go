// internal/kite/ruling58_test.go
package kite

import (
	"context"
	"testing"
	"time"

	"tradeloom/internal/secrets"
)

// Ruling 58 (1): an endSession for an older session that is waiting on feedMu while a paste login
// commits must not wipe the new session (state, in-memory token, stored token, feed).
func TestEndSession_WaitingOnFeedMu_DoesNotWipeNewLogin(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	if _, err := e.sess.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	old := TokenRecord{AccessToken: "acc-old-111", UserID: "AB1234", SessionID: "sess-old"}

	e.sess.feedMu.Lock() // e.g. a probe of the old session holding feedMu around Feed.Start
	ended := make(chan struct{})
	go func() { defer close(ended); e.sess.endSession(old, CodeSessionExpired) }()
	logged := make(chan struct{})
	go func() {
		defer close(logged)
		if _, err := e.sess.CompleteLogin(context.Background(), "rtok12345678"); err != nil {
			t.Error(err)
		}
	}()
	eventually(t, "exchange sent", func() bool { return e.auth.exchangeCount() == 1 })
	// Give a commit that does not take feedMu the chance to finish first (that is the bug); a commit
	// that does take feedMu stays blocked here, and the deadline simply passes.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) && e.sess.State().State != StateReady {
		time.Sleep(2 * time.Millisecond)
	}
	e.sess.feedMu.Unlock()
	<-ended
	<-logged

	st := e.sess.State()
	if st.State != StateReady || st.SessionID != "sess-1" {
		t.Fatalf("new session wiped: %+v", st)
	}
	if e.tokens.get() != "acc-live-777" || !e.secrets.has(secrets.KeyAccessToken) || e.lim.isPaused() {
		t.Fatalf("new token cleared (mem %q, stored %v) or limiter paused", e.tokens.get(), e.secrets.has(secrets.KeyAccessToken))
	}
	eventually(t, "feed started for the new session", func() bool { return e.feed.startCount() == 1 })
	if n := e.feed.stopCount(); n != 0 {
		// A stop that ran before the commit is harmless only if the feed started afterwards.
		if ev := e.feed.eventLog(); ev[len(ev)-1] != "start" {
			t.Fatalf("feed events %v", ev)
		}
	}
}

// Ruling 58 (2): without Run, the login timer and the listener still stop on Close, because the run
// context is a cancellable base from NewSession, never context.Background().
func TestClose_BeforeRun_CancelsLoginTimer(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	if _, err := e.sess.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !e.clock.WaitTimers(1, 3*time.Second) {
		t.Fatal("login timer not armed")
	}
	if e.sess.ctx() == context.Background() {
		t.Fatal("run context is context.Background()")
	}
	e.sess.Close()
	eventually(t, "login timer stopped", func() bool { return e.clock.PendingTimers() == 0 })
}
