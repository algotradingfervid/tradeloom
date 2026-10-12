// internal/kite/login.go
package kite

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/bus"
	"tradeloom/internal/logging"
	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
)

// LoginStart tells the UI what to open and whether the loopback listener is up.
type LoginStart struct {
	LoginURL      string `json:"loginUrl"`
	RedirectURL   string `json:"redirectUrl"`
	ListenerOK    bool   `json:"listenerOk"`
	ListenerError string `json:"listenerError"`
}

const (
	msgNoCredentials = "Set up your Kite API key and secret first."
	msgCancelled     = "Kite reports the login was not completed. Click \"Log in with Kite\" to try again."
	// maxKiteMessage bounds the Kite-supplied text shown to the user (Task 3 minor: never echo untruncated).
	maxKiteMessage = 160
)

func newNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// BeginLogin starts (or restarts) a login attempt: a fresh state nonce, the loopback listener and the login URL.
// The caller opens LoginURL in the system browser (internal/app does it with runtime.BrowserOpenURL).
func (s *Session) BeginLogin(_ context.Context) (LoginStart, error) {
	ok, err := s.configured()
	if err != nil {
		s.storeFailed("begin login", err)
		return LoginStart{}, &LoginError{Code: CodeSecretStore, Message: msgSecretStore}
	}
	if !ok {
		return LoginStart{}, &LoginError{Code: CodeNoCredentials, Message: msgNoCredentials}
	}
	apiKey := s.apiKey()
	port := settings.Get(s.d.Settings, SettingRedirectPort)
	nonce, err := newNonce()
	if err != nil {
		return LoginStart{}, err
	}
	start := LoginStart{LoginURL: s.d.Auth(apiKey).LoginURL(nonce), RedirectURL: RedirectURL(port)}

	s.mu.Lock()
	if s.st.State == StateReady {
		s.mu.Unlock()
		return LoginStart{}, &LoginError{Code: CodeAlreadyLoggedIn, Message: "Already logged in to Kite."}
	}
	if s.st.State != StateLoggingIn {
		s.prior = s.st.State
	}
	s.nonces[nonce] = true
	var stale *loopback
	if s.lb != nil && s.lb.port != port {
		stale, s.lb = s.lb, nil
	}
	if s.lb == nil {
		lb, lerr := startLoopback(port, s.handleCallback, s.d.Log)
		if lerr != nil {
			start.ListenerError = listenerHint(port, lerr)
		} else {
			s.lb = lb
		}
	}
	start.ListenerOK = s.lb != nil
	s.gen++
	gen := s.gen
	st := SessionState{State: StateLoggingIn, DataPlan: PlanUnknown}
	s.st = st
	runCtx := s.runCtx
	s.mu.Unlock()
	if stale != nil {
		stale.Close()
	}

	bus.Publish(s.d.Bus, TopicSessionState, st)
	timeout := s.d.Clock.At(s.d.Clock.Now().Add(s.d.ListenerTimeout))
	logging.Go("kite.login.timeout", func() {
		select {
		case <-runCtx.Done():
			timeout.Stop()
			s.closeListener(gen)
		case <-timeout.C():
			s.onLoginTimeout(gen)
		}
	})
	s.d.Log.Info("kite login started", "listener", start.ListenerOK, "port", port)
	return start, nil
}

// closeListener closes the loopback listener when attempt gen is still the latest one.
func (s *Session) closeListener(gen int) {
	s.mu.Lock()
	var lb *loopback
	if gen == s.gen {
		lb, s.lb = s.lb, nil
	}
	s.mu.Unlock()
	if lb != nil {
		lb.Close()
	}
}

// onLoginTimeout ends attempt gen after ListenerTimeout: the listener closes, and the state falls back
// only if the login is still pending (a finished or failed login keeps its state).
func (s *Session) onLoginTimeout(gen int) {
	s.mu.Lock()
	if gen != s.gen {
		s.mu.Unlock()
		return
	}
	lb := s.lb
	s.lb = nil
	pending := s.st.State == StateLoggingIn
	st := SessionState{State: s.prior, DataPlan: PlanUnknown, ErrorCode: CodeListenerTimeout,
		ErrorMessage: "The login window timed out. If you already logged in, paste the address from the browser's address bar below; otherwise click \"Log in with Kite\" again."}
	if pending {
		s.st = st
	}
	s.mu.Unlock()
	if lb != nil {
		lb.Close()
	}
	if pending {
		bus.Publish(s.d.Bus, TopicSessionState, st)
	}
}

// CancelLogin abandons a pending login and closes the listener.
func (s *Session) CancelLogin() SessionState {
	s.mu.Lock()
	if s.st.State != StateLoggingIn {
		st := s.st
		s.mu.Unlock()
		return st
	}
	s.gen++
	lb := s.lb
	s.lb = nil
	st := SessionState{State: s.prior, DataPlan: PlanUnknown}
	s.st = st
	s.mu.Unlock()
	if lb != nil {
		lb.Close()
	}
	bus.Publish(s.d.Bus, TopicSessionState, st)
	return st
}

// handleCallback serves the loopback redirect. Ruling 36: the callback must carry a state nonce issued
// in this run (Kite echoes it through redirect_params); a missing or unknown state is a forged local
// request and is refused before anything else is read. Stateless redirects are accepted only by paste.
func (s *Session) handleCallback(r *http.Request) (int, string) {
	state := r.URL.Query().Get("state")
	s.mu.Lock()
	known := state != "" && s.nonces[state]
	ctx := s.runCtx
	s.mu.Unlock()
	if !known {
		s.d.Log.Warn("loopback callback refused", "reason", "unknown or missing state")
		return http.StatusBadRequest, resultPage("Login link not recognised",
			"This login was started by an earlier Tradeloom run or another app. Return to Tradeloom and click \"Log in with Kite\" again.")
	}
	red, err := ParseRedirect("?" + r.URL.RawQuery)
	if err != nil {
		return http.StatusBadRequest, resultPage("Login not completed", safeRedirectError(err).Error())
	}
	s.mu.Lock()
	already := s.done[red.RequestToken]
	s.mu.Unlock()
	if already {
		return http.StatusOK, resultPage(pageTitleOK, pageMsgOK)
	}
	if _, err := s.exchange(ctx, red.RequestToken); err != nil {
		return http.StatusBadRequest, resultPage("Login failed", err.Error())
	}
	return http.StatusOK, resultPage(pageTitleOK, pageMsgOK)
}

// safeRedirectError replaces ParseRedirect's cancelled-login message, which quotes Kite's raw status
// value, with fixed text (Task 3 minor). Other ParseRedirect errors are fixed text already.
func safeRedirectError(err error) error {
	var le *LoginError
	if errors.As(err, &le) && le.Code == CodeLoginCancelled {
		return &LoginError{Code: CodeLoginCancelled, Message: msgCancelled}
	}
	return err
}

// CompleteLogin is the paste fallback: a full redirect URL, its query, or a bare request_token.
// It is accepted in any non-ready state, whatever its state nonce: pasting is an explicit user action.
func (s *Session) CompleteLogin(ctx context.Context, pasted string) (SessionState, error) {
	red, err := ParseRedirect(pasted)
	if err != nil {
		return s.State(), safeRedirectError(err)
	}
	if red.State != "" {
		s.mu.Lock()
		known := s.nonces[red.State]
		s.mu.Unlock()
		if !known {
			s.d.Log.Info("pasted redirect carries a state from another login attempt; accepted because pasting is explicit")
		}
	}
	return s.exchange(ctx, red.RequestToken)
}

// exchange trades a request_token for an access token. Lock order: exMu, then mu (afterReady takes
// feedMu on its own goroutine, never under exMu).
func (s *Session) exchange(ctx context.Context, requestToken string) (SessionState, error) {
	s.exMu.Lock()
	defer s.exMu.Unlock()
	logging.RegisterSecret(requestToken)

	s.mu.Lock()
	if s.done[requestToken] {
		st := s.st
		s.mu.Unlock()
		return st, nil
	}
	if s.st.State == StateReady {
		st := s.st
		s.mu.Unlock()
		return st, &LoginError{Code: CodeAlreadyLoggedIn, Message: "Already logged in to Kite."}
	}
	s.mu.Unlock()

	apiKey := s.apiKey()
	secret, err := s.secret()
	if err != nil {
		s.storeFailed("read api secret", err)
		le := &LoginError{Code: CodeSecretStore, Message: msgSecretStore}
		s.failLogin(le)
		return s.State(), le
	}
	if apiKey == "" || secret == "" {
		return s.State(), &LoginError{Code: CodeNoCredentials, Message: msgNoCredentials}
	}
	auth := s.d.Auth(apiKey)
	us, err := auth.GenerateSession(ctx, requestToken, secret)
	if err != nil {
		le := mapExchangeError(scrubErr(err, requestToken, secret))
		s.failLogin(le)
		return s.State(), le
	}
	logging.RegisterSecret(us.AccessToken)
	now := s.d.Clock.Now()
	rec := TokenRecord{AccessToken: us.AccessToken, UserID: us.UserID, SessionID: s.d.NewID(),
		LoggedInAtMs: now.UnixMilli(), ExpiresAtMs: NextExpiry(now).UnixMilli()}
	b, err := json.Marshal(rec)
	if err != nil {
		return s.State(), err
	}
	if err := s.d.Secrets.Set(secrets.KeyAccessToken, string(b)); err != nil {
		s.storeFailed("save access token", err)
		if ierr := auth.Invalidate(ctx, us.AccessToken); ierr != nil {
			s.d.Log.Warn("invalidating the unsaved access token failed", "kind", errKind(ierr))
		}
		le := &LoginError{Code: CodeSecretStore, Message: msgSecretStore}
		s.failLogin(le)
		return s.State(), le
	}
	if err := s.insertRow(ctx, rec, PlanUnknown); err != nil {
		s.d.Log.Error("recording the session failed", "err", err)
	}
	s.d.Tokens.SetAPIKey(apiKey)
	s.d.Tokens.SetAccessToken(rec.AccessToken)
	s.d.Limiter.Resume()
	s.mu.Lock()
	s.done[requestToken] = true
	s.rec = &rec
	s.mu.Unlock()
	st := s.set(readyState(rec, PlanUnknown, true, false))
	s.d.Log.Info("kite login complete", "user", MaskUserID(rec.UserID), "expires_at_ms", rec.ExpiresAtMs)
	s.afterReady(apiKey, rec)
	return st, nil
}

func (s *Session) failLogin(le *LoginError) {
	s.mu.Lock()
	base := s.st.State
	if base == StateLoggingIn || base == StateReady {
		base = s.prior
	}
	st := SessionState{State: base, DataPlan: PlanUnknown, ErrorCode: le.Code, ErrorMessage: le.Message}
	s.st = st
	s.mu.Unlock()
	bus.Publish(s.d.Bus, TopicSessionState, st)
	s.d.Log.Warn("kite login failed", "code", le.Code)
}

// scrub removes the request token and the API secret from user-facing text built from a Kite reply.
func scrub(msg string, values ...string) string {
	for _, v := range values {
		if v != "" {
			msg = strings.ReplaceAll(msg, v, "••••")
		}
	}
	return msg
}

// scrubErr returns err with the given values removed from a Kite error's message (before any truncation).
func scrubErr(err error, values ...string) error {
	var ke kiteconnect.Error
	if errors.As(err, &ke) {
		ke.Message = scrub(ke.Message, values...)
		return ke
	}
	return err
}

// kiteText bounds the Kite-supplied message shown to the user.
func kiteText(m string) string {
	if utf8.RuneCountInString(m) <= maxKiteMessage {
		return m
	}
	r := []rune(m)
	return string(r[:maxKiteMessage]) + "…"
}

// mapExchangeError turns a GenerateSession failure into user guidance. A TokenException here means the
// request_token is spent or stale; it is NOT a session expiry (Review Focus 2).
func mapExchangeError(err error) *LoginError {
	var ke kiteconnect.Error
	if errors.As(err, &ke) {
		msg := strings.ToLower(ke.Message)
		switch {
		case ke.ErrorType == kiteconnect.TokenError:
			return &LoginError{Code: CodeRequestTokenInvalid, Message: "This login link was already used or has expired (Kite request tokens last only a few minutes). Click \"Log in with Kite\" again."}
		case ke.ErrorType == kiteconnect.InputError && (strings.Contains(msg, "checksum") || strings.Contains(msg, "api_key") || strings.Contains(msg, "api key")):
			return &LoginError{Code: CodeCredentialsRejected, Message: "Kite rejected the API key or secret (" + kiteText(ke.Message) + "). Check them in Settings → Kite connection."}
		case ke.ErrorType == kiteconnect.NetworkError:
			return &LoginError{Code: CodeNetwork, Message: "Could not reach Kite. Check the internet connection and try again."}
		default:
			return &LoginError{Code: CodeKiteError, Message: "Kite refused the login: " + kiteText(ke.Message)}
		}
	}
	return &LoginError{Code: CodeNetwork, Message: "Could not reach Kite. Check the internet connection and try again."}
}
