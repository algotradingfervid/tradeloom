// internal/kite/session_relay.go
package kite

import "sync"

// SessionErrorRelay is the late-bound target of Options.OnSessionError: the Client is built before the
// Session exists (the Session needs the Client), so cmd/tradeloom passes relay.Handle to kite.New and
// calls relay.Set(sess.HandleTokenError) once the Session is built.
type SessionErrorRelay struct {
	mu sync.Mutex
	fn func(error)
}

// Set installs (or replaces) the target.
func (r *SessionErrorRelay) Set(fn func(error)) {
	r.mu.Lock()
	r.fn = fn
	r.mu.Unlock()
}

// Handle forwards a session error to the target; before Set it drops the error. The *Err is passed
// through unchanged, so its TokenGen (Ruling 70) reaches Session.HandleTokenError.
func (r *SessionErrorRelay) Handle(e *Err) {
	r.mu.Lock()
	fn := r.fn
	r.mu.Unlock()
	if fn != nil && e != nil {
		fn(e)
	}
}
