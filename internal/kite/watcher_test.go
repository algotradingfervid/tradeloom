// internal/kite/watcher_test.go
package kite

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/secrets"
)

func runWatcher(t *testing.T, e *testEnv) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go e.sess.Run(ctx)
}

func assertExpired(t *testing.T, e *testEnv, code string) {
	t.Helper()
	// endSession (Task 6) leaves the ready state first (so an in-flight probe sees the session as no
	// longer current) and then runs the side effects, so wait for both before asserting.
	eventually(t, "expired", func() bool {
		return e.sess.State().State == StateExpired && e.feed.stopCount() > 0 && e.lim.isPaused() &&
			e.tokens.get() == "" && !e.secrets.has(secrets.KeyAccessToken)
	})
	st := e.sess.State()
	if st.ErrorCode != code || st.ErrorMessage != "Kite session expired — log in to continue." {
		t.Fatalf("state %+v", st)
	}
	if e.feed.stopCount() != 1 {
		t.Fatalf("ticker stopped %d times, want 1", e.feed.stopCount())
	}
	if !e.lim.isPaused() || e.tokens.get() != "" || e.secrets.has(secrets.KeyAccessToken) {
		t.Fatal("limiter must pause, token must be cleared and deleted")
	}
}

// FR-ACCT-006 AC3, Review Focus 3.
func TestExpiry_At0600_WithTickerRunning(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	st := loginViaPaste(t, e)
	runWatcher(t, e)
	e.clock.Set(ist(2026, 10, 13, 5, 59))
	// Ruling 58 (3): the watcher re-arms its timer only after its check at 05:59 has run.
	if !e.clock.WaitTimers(1, 3*time.Second) {
		t.Fatal("watcher did not re-arm after 05:59")
	}
	if e.sess.State().State != StateReady {
		t.Fatal("expired before 06:00")
	}
	e.clock.Set(ist(2026, 10, 13, 6, 0))
	assertExpired(t, e, CodeSessionExpired)
	// endRow runs after the token delete inside endSession.
	eventually(t, "session row ended", func() bool {
		row, _ := e.repo.Get(context.Background(), st.SessionID)
		return row.EndReason == CodeSessionExpired
	})
	assertNoSecondStop(t, e)
}

// assertNoSecondStop lets the watcher poll once more after the expiry and checks that the end-of-session
// side effects did not run again (Ruling 58 (3)).
func assertNoSecondStop(t *testing.T, e *testEnv) {
	t.Helper()
	if !e.clock.WaitTimers(1, 3*time.Second) {
		t.Fatal("watcher did not re-arm after the expiry")
	}
	e.clock.Advance(expiryPoll)
	if !e.clock.WaitTimers(1, 3*time.Second) {
		t.Fatal("watcher did not re-arm after the extra poll")
	}
	if n := e.feed.stopCount(); n != 1 {
		t.Fatalf("ticker stopped %d times after one more poll, want 1", n)
	}
}

// Review Focus 3: the machine was suspended across 06:00; monotonic timers are late, the 5 s check is not.
func TestExpiry_ClockJumpAfterSuspend(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 22, 0))
	e.configure()
	loginViaPaste(t, e)
	runWatcher(t, e)
	e.clock.Set(ist(2026, 10, 13, 7, 30))
	assertExpired(t, e, CodeSessionExpired)
	assertNoSecondStop(t, e)
}

// FR-ACCT-006 AC5: any TokenException enters the expired state; other errors do not.
func TestTokenException_EntersExpiredOnce(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	loginViaPaste(t, e)
	e.sess.HandleTokenError(APIError(kiteconnect.InputError, 400, "Invalid `from` date"))
	if e.sess.State().State != StateReady {
		t.Fatal("non-token error ended the session")
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.sess.HandleTokenError(APIError(kiteconnect.TokenError, 403, "Incorrect `api_key` or `access_token`."))
		}()
	}
	wg.Wait()
	assertExpired(t, e, CodeTokenRejected)
}

// relayProfiles simulates the Client's OnSessionError relay: a rejected Profile call also reaches
// HandleTokenError, synchronously, before the call returns to Restore.
type relayProfiles struct {
	sess *Session
	err  error
}

func (r *relayProfiles) Profile(context.Context) (kiteconnect.UserProfile, error) {
	r.sess.HandleTokenError(r.err)
	return kiteconnect.UserProfile{}, r.err
}

// Carry-forward (Task 6): Restore's own endSession and the client's relay both see the rejection; the
// end-of-session side effects run once.
func TestTokenException_DuringRestore_EndsOnce(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	e.seedToken(ist(2026, 10, 12, 8, 0), true)
	e.sess.d.Profiles = &relayProfiles{sess: e.sess, err: APIError(kiteconnect.TokenError, 403, "Incorrect `api_key` or `access_token`.")}
	st := e.sess.Restore(context.Background())
	if st.State != StateExpired || st.ErrorCode != CodeTokenRejected {
		t.Fatalf("state %+v", st)
	}
	assertExpired(t, e, CodeTokenRejected)
	// A late relay call after the session ended is ignored as well.
	e.sess.HandleTokenError(APIError(kiteconnect.TokenError, 403, "Incorrect `api_key` or `access_token`."))
	if e.feed.stopCount() != 1 {
		t.Fatalf("ticker stopped %d times, want 1", e.feed.stopCount())
	}
}

// A token error while no session is ready (logged out, mid-login) changes nothing.
func TestTokenException_IgnoredWhenNotReady(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	e.sess.HandleTokenError(APIError(kiteconnect.TokenError, 403, "Incorrect `api_key` or `access_token`."))
	if st := e.sess.State(); st.State != StateLoggedOut || st.ErrorCode != "" {
		t.Fatalf("state %+v", st)
	}
	if e.feed.stopCount() != 0 {
		t.Fatal("feed stopped without a session")
	}
	e.sess.HandleTokenError(nil)
}

// Task 7 concern: Run sets the context the login-timeout goroutine waits on, so shutdown (Run's ctx
// cancelled) stops the pending 10-minute login timer and closes the loopback listener.
func TestRun_ShutdownCancelsPendingLoginTimer(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	go func() { e.sess.Run(ctx); close(stopped) }()
	if !e.clock.WaitTimers(1, 3*time.Second) {
		t.Fatal("watcher never armed its timer")
	}
	eventually(t, "run context installed", func() bool { return e.sess.ctx() == ctx })
	port := freePort(t)
	setPort(t, e, port)
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil || !ls.ListenerOK {
		t.Fatalf("begin login: %+v %v", ls, err)
	}
	if !e.clock.WaitTimers(2, 3*time.Second) {
		t.Fatal("login timer not armed")
	}
	cancel()
	<-stopped
	eventually(t, "login timer stopped", func() bool { return e.clock.PendingTimers() == 0 })
	eventually(t, "listener closed", func() bool {
		ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			return false
		}
		_ = ln.Close()
		return true
	})
}

// Close is idempotent and safe without a listener.
func TestClose_Idempotent(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.sess.Close()
	e.sess.Close()
}
