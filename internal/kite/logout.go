// internal/kite/logout.go
package kite

import (
	"context"
	"errors"

	"tradeloom/internal/bus"
	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
)

const CodeInvalidPort = "invalid_port"

const (
	msgNotInvalidated = "Logged out of Tradeloom. Kite could not be reached to invalidate the token; it stops working at 06:00 IST."
	msgTokenKept      = "The saved access token could not be removed: " + msgSecretStore + ", then log out again."
)

type LogoutResult struct {
	RemoteInvalidated bool   `json:"remoteInvalidated"`
	Message           string `json:"message"`
}

// Logout implements FR-ACCT-004: invalidate at Kite, delete the token, stop the ticker, pause all Kite traffic.
// Order: close the listener → stop the ticker → pause the limiter → DELETE /session/token (best effort; the
// local logout always completes) → end the session row → delete the token secret → publish logged_out.
//
// Lock order exMu → feedMu → mu. The session is claimed (rec taken, state left) under feedMu, so an expiry
// or a token rejection racing with the logout runs the end-of-session side effects once: whichever claims
// rec first. feedMu is released before the Kite call. A token Delete that fails (e.g. times out behind a
// hung Set of the same key) is reported with CodeSecretStore, never claimed as success; logging out again
// retries it.
func (s *Session) Logout(ctx context.Context) (LogoutResult, error) {
	s.exMu.Lock()
	defer s.exMu.Unlock()

	s.feedMu.Lock()
	s.mu.Lock()
	rec := s.rec
	s.rec = nil
	lb := s.lb
	s.lb = nil
	s.gen++
	gen := s.gen
	s.done = map[string]bool{}
	s.nonces = map[string]nonceBinding{}
	s.st = SessionState{State: StateLoggedOut, DataPlan: PlanUnknown} // not ready any more; published below
	s.mu.Unlock()
	if lb != nil {
		lb.Close()
	}
	if rec != nil {
		s.stopTraffic("logged out")
	} else {
		// No live session (never logged in, or it already ended and ran its side effects).
		s.d.Tokens.SetAccessToken("")
	}
	s.feedMu.Unlock()

	res := LogoutResult{}
	if rec != nil {
		if err := s.d.Auth(s.apiKey()).Invalidate(ctx, rec.AccessToken); err != nil {
			res.Message = msgNotInvalidated
			s.d.Log.Warn("invalidating the access token failed", "kind", errKind(err))
		} else {
			res.RemoteInvalidated = true
		}
		s.endRow(rec.SessionID, "logout")
	}
	derr := s.deleteToken()
	configured, cerr := s.configured()
	if cerr != nil {
		s.storeFailed("read api secret after logout", cerr)
	}
	st := idle(configured, cerr, "", "")
	if derr != nil {
		s.storeFailed("delete access token at logout", derr)
		st.ErrorCode, st.ErrorMessage = CodeSecretStore, msgSecretStore
		if res.Message != "" {
			res.Message += " "
		}
		res.Message += msgTokenKept
	}
	s.mu.Lock()
	current := s.gen == gen // a login begun meanwhile owns the state now
	if current {
		s.st = st
	}
	s.mu.Unlock()
	if current {
		bus.Publish(s.d.Bus, TopicSessionState, st)
	}
	s.d.Log.Info("kite logout", "remote_invalidated", res.RemoteInvalidated, "token_deleted", derr == nil)
	if derr != nil {
		return res, &LoginError{Code: CodeSecretStore, Message: msgSecretStore}
	}
	return res, nil
}

// RemoveCredentials implements FR-ACCT-005 "Remove credentials": logout, then delete the key, secret and token.
func (s *Session) RemoveCredentials(ctx context.Context) (SessionState, error) {
	if s.State().State == StateLoggingIn {
		s.CancelLogin()
	}
	if _, err := s.Logout(ctx); err != nil {
		return s.State(), err
	}
	if err := s.d.Secrets.Delete(secrets.KeyAPISecret); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		s.storeFailed("delete api secret", err)
		return s.State(), &LoginError{Code: CodeSecretStore, Message: msgSecretStore}
	}
	if err := settings.Set(ctx, s.d.Settings, SettingAPIKey, ""); err != nil {
		return s.State(), err
	}
	s.d.Tokens.SetAPIKey("")
	s.d.Log.Info("kite credentials removed")
	return s.set(SessionState{State: StateNoCredentials, DataPlan: PlanUnknown}), nil
}

// portGuidance tells the user what to change in the Kite developer console after a port change (Ruling 57).
func portGuidance(port int) string {
	return "Then set the redirect URL of your app in the Kite developer console to " + RedirectURL(port) + "."
}

// SetRedirectPort changes the loopback port; the user must register the new RedirectURL in the developer console.
func (s *Session) SetRedirectPort(port int) (CredentialsInfo, error) {
	invalid := &LoginError{Code: CodeInvalidPort, Message: "Choose a port between 1024 and 65535. Then set the redirect URL of your app in the Kite developer console to http://127.0.0.1:<port>" + CallbackPath + " with that port."}
	if ValidatePort(port) != nil {
		return s.CredentialsInfo(), invalid
	}
	if s.State().State == StateLoggingIn {
		return s.CredentialsInfo(), &LoginError{Code: CodeLoginBusy, Message: "Finish or cancel the current login before changing the port. " + portGuidance(port)}
	}
	if err := settings.Set(context.Background(), s.d.Settings, SettingRedirectPort, port); err != nil {
		var ve *settings.ValidationError
		if errors.As(err, &ve) {
			return s.CredentialsInfo(), invalid
		}
		return s.CredentialsInfo(), err
	}
	s.d.Log.Info("kite redirect port changed", "port", port, "guidance", portGuidance(port))
	return s.CredentialsInfo(), nil
}
