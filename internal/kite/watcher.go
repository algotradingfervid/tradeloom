// internal/kite/watcher.go
package kite

import (
	"context"
	"errors"
	"time"
)

// Run is the session.Watcher of ARCH §3.1. It wakes at the token expiry instant (rec.ExpiresAtMs, the
// NextExpiry of the login instant) and at least every 5 s, both on the app clock, so a clock jump
// (suspend/resume, offset correction) is noticed within 5 s.
//
// Run also installs ctx as the session's run context: the login-timeout goroutines (BeginLogin) and the
// capability probe wait on it, so cancelling ctx at shutdown stops pending login timers and probes.
// When Run returns, the loopback listener is closed.
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
		s.expire(CodeSessionExpired, 0)
	}
}

// HandleTokenError is the target of kite.SessionErrorRelay (P0-2's kite.Options.OnSessionError): a
// TokenException ends the session (FR-ACCT-006 AC5). The *kite.Err it receives unwraps to the kiteconnect.Error.
// A session that is no longer ready is ignored: Restore ends a rejected session itself, and the relay
// fires for the same rejection, so the end-of-session side effects must run only once.
//
// Ruling 70: an error whose Err.TokenGen names a token other than the current session's is stale (e.g. a
// day-1 call answered after the day-2 login) and is ignored, logged at Info with the generations only.
// An error without a generation (TokenGen 0: not a *kite.Err, or one built outside Client.call) and a
// session whose generation is unknown (a TokenSink without TokenGen) keep the earlier behaviour: the
// error ends the ready session.
func (s *Session) HandleTokenError(err error) {
	if err != nil && isTokenError(err) {
		s.expire(CodeTokenRejected, errTokenGen(err))
	}
}

// errTokenGen is the token generation an error carries (Err.TokenGen), or 0 when it carries none.
func errTokenGen(err error) uint64 {
	var ke *Err
	if errors.As(err, &ke) {
		return ke.TokenGen
	}
	return 0
}

// expire claims the ready→expired transition exactly once, then performs the side effects. tokGen is the
// token generation the triggering error carries (0 = unknown or not from a call); a known generation
// other than the current session's leaves the session alone (Ruling 70). The check runs under feedMu and
// mu, the locks a login commit (rec and tokGen together) holds, so it cannot see a half-committed session.
// Ruling 58 (1): it takes feedMu before its mu claim, so no login can commit between the claim and the
// side effects. It never takes exMu, because the relay can call HandleTokenError from inside a Kite call
// made while exMu is held; feedMu is never held across a Kite call, so that cannot deadlock.
func (s *Session) expire(code string, tokGen uint64) {
	s.feedMu.Lock()
	defer s.feedMu.Unlock()
	s.mu.Lock()
	if s.st.State != StateReady || s.rec == nil {
		s.mu.Unlock()
		return
	}
	if tokGen != 0 && s.tokGen != 0 && tokGen != s.tokGen {
		cur, sid := s.tokGen, s.rec.SessionID
		s.mu.Unlock()
		s.d.Log.Info("kite token error for an earlier access token ignored; the current session is kept",
			"session_id", sid, "token_gen", tokGen, "current_gen", cur)
		return
	}
	rec := *s.rec
	s.rec = nil
	s.st.State = StateExpired
	s.mu.Unlock()
	s.endSessionLocked(rec, code)
}

// Close shuts the loopback listener and cancels the base run context (app shutdown). It is idempotent.
func (s *Session) Close() {
	s.cancelBase()
	s.mu.Lock()
	lb := s.lb
	s.lb = nil
	s.mu.Unlock()
	if lb != nil {
		lb.Close()
	}
}
