> **P0-3 Session & Service Layer Implementation Plan — Task 8.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 8: Session lifecycle — 06:00 expiry watcher and `TokenException` handling

**Files:**
- Create: `internal/kite/watcher.go`
- Test: `internal/kite/watcher_test.go`

**Interfaces:**
- Consumes: Task 6 (`endSession`, `SessionErrorRelay`), Task 7 test helper `loginViaPaste`, P0-2 `clock.Clock.At` (`clock.Timer`), `clock.Fake.Set/Advance` (both fire due timers).
- Produces:
  ```go
  func (s *Session) Run(ctx context.Context)      // blocks; wakes at ExpiresAt and every 5 s (suspend-safe)
  func (s *Session) HandleTokenError(err error)   // target of kite.SessionErrorRelay.Set (P0-2 kite.Options.OnSessionError)
  func (s *Session) Close()                       // closes the loopback listener
  ```

- [ ] **Step 1: Write the failing tests**

```go
// internal/kite/watcher_test.go
package kite

import (
	"context"
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
	eventually(t, "expired", func() bool { return e.sess.State().State == StateExpired })
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
	time.Sleep(30 * time.Millisecond)
	if e.sess.State().State != StateReady {
		t.Fatal("expired before 06:00")
	}
	e.clock.Set(ist(2026, 10, 13, 6, 0))
	assertExpired(t, e, CodeSessionExpired)
	row, _ := e.repo.Get(context.Background(), st.SessionID)
	if row.EndReason != CodeSessionExpired {
		t.Fatalf("row %+v", row)
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestExpiry|TestTokenException' -v`
Expected: FAIL to compile with `undefined: (*Session).Run`, `undefined: (*Session).HandleTokenError`.

- [ ] **Step 3: Write the implementation**

```go
// internal/kite/watcher.go
package kite

import (
	"context"
	"time"
)

// Run is the session.Watcher of ARCH §3.1. It wakes at the token expiry instant and at least every 5 s, so a
// clock jump (suspend/resume, offset correction) is noticed within 5 s.
func (s *Session) Run(ctx context.Context) {
	s.mu.Lock()
	s.runCtx = ctx
	s.mu.Unlock()
	defer s.Close()
	for {
		next := s.d.Clock.Now().Add(expiryPoll)
		s.mu.Lock()
		if s.rec != nil && s.st.State == StateReady {
			if exp := time.UnixMilli(s.rec.ExpiresAtMs); exp.Before(next) {
				next = exp
			}
		}
		s.mu.Unlock()
		wake := s.d.Clock.At(next)
		select {
		case <-ctx.Done():
			wake.Stop()
			return
		case <-wake.C():
		}
		s.checkExpiry()
	}
}

func (s *Session) checkExpiry() {
	s.mu.Lock()
	due := s.rec != nil && s.st.State == StateReady && s.d.Clock.Now().UnixMilli() >= s.rec.ExpiresAtMs
	s.mu.Unlock()
	if due {
		s.expire(CodeSessionExpired)
	}
}

// HandleTokenError is the target of kite.SessionErrorRelay (P0-2's kite.Options.OnSessionError): any
// TokenException ends the session (FR-ACCT-006 AC5). The *kite.Err it receives unwraps to the kiteconnect.Error.
func (s *Session) HandleTokenError(err error) {
	if err != nil && isTokenError(err) {
		s.expire(CodeTokenRejected)
	}
}

// expire claims the ready→expired transition exactly once, then performs the side effects.
func (s *Session) expire(code string) {
	s.mu.Lock()
	if s.st.State != StateReady || s.rec == nil {
		s.mu.Unlock()
		return
	}
	rec := *s.rec
	s.rec = nil
	s.st.State = StateExpired
	s.mu.Unlock()
	s.endSession(rec, code)
}

// Close shuts the loopback listener (app shutdown).
func (s *Session) Close() {
	s.mu.Lock()
	lb := s.lb
	s.lb = nil
	s.mu.Unlock()
	if lb != nil {
		lb.Close()
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/kite/ -run 'TestExpiry|TestTokenException' -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/kite/watcher.go internal/kite/watcher_test.go
git commit -m "feat(kite): 06:00 IST expiry watcher and TokenException handling

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
