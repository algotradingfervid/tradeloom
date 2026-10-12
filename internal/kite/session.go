// internal/kite/session.go
package kite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/logging"
	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

const (
	defaultListenerTimeout = 10 * time.Minute
	expiryPoll             = 5 * time.Second
	probeAttempts          = 3
	probeRetry             = 30 * time.Second
	msgExpired             = "Kite session expired — log in to continue."
)

// secretsTimeout bounds every secret-store call made by the Session: a locked keyring collection can
// block on an unlock prompt that never appears (P0-1 carry-forward), and these calls run on the UI path.
// A variable only so tests can shorten it.
var secretsTimeout = secrets.ProbeTimeout

// endedInvalidateTimeout bounds the best-effort Kite invalidation of an ended session's token at startup
// (Ruling 63). A variable only so tests can shorten it.
var endedInvalidateTimeout = 5 * time.Second

// msgSecretStore is the fixed, user-facing text for any secret-store failure (the cause is only logged).
const msgSecretStore = "secret store not answering — unlock it and retry"

// boundedSecrets runs each call of the wrapped store on its own goroutine and gives up after
// secretsTimeout with an error wrapping secrets.ErrUnavailable. It is single-flight per key: while a
// call for a key is in flight (e.g. hung on a locked keyring), later calls for that key wait for it
// and never start a second goroutine, so goroutines cannot pile up and a timed-out Set can never land
// after a later Delete of the same key. A call that answers late still completes; its result is discarded.
type boundedSecrets struct {
	next secrets.Store

	mu   sync.Mutex
	busy map[secrets.Key]chan struct{} // closed when the in-flight call for the key returns
}

func newBoundedSecrets(next secrets.Store) *boundedSecrets {
	if b, ok := next.(*boundedSecrets); ok {
		return b
	}
	return &boundedSecrets{next: next, busy: map[secrets.Key]chan struct{}{}}
}

func errSecretsTimeout() error {
	return fmt.Errorf("%w: the secret store did not answer within %v", secrets.ErrUnavailable, secretsTimeout)
}

// do runs fn for key k under the per-key single-flight slot, bounded by secretsTimeout overall.
func (b *boundedSecrets) do(k secrets.Key, name string, fn func() (string, error)) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), secretsTimeout)
	defer cancel()
	var slot chan struct{}
	for slot == nil {
		b.mu.Lock()
		inflight, busy := b.busy[k]
		if !busy {
			slot = make(chan struct{})
			b.busy[k] = slot
		}
		b.mu.Unlock()
		if busy {
			select {
			case <-inflight:
			case <-ctx.Done():
				return "", errSecretsTimeout()
			}
		}
	}
	type result struct {
		v   string
		err error
	}
	ch := make(chan result, 1)
	logging.Go(name, func() {
		v, err := fn()
		b.mu.Lock()
		delete(b.busy, k)
		close(slot)
		b.mu.Unlock()
		ch <- result{v, err}
	})
	select {
	case r := <-ch:
		return r.v, r.err
	case <-ctx.Done():
		return "", errSecretsTimeout()
	}
}

func (b *boundedSecrets) Get(k secrets.Key) (string, error) {
	v, err := b.do(k, "kite.secrets.get", func() (string, error) { return b.next.Get(k) })
	if err == nil {
		logging.RegisterSecret(v)
	}
	return v, err
}

func (b *boundedSecrets) Set(k secrets.Key, v string) error {
	logging.RegisterSecret(v)
	_, err := b.do(k, "kite.secrets.set", func() (string, error) { return "", b.next.Set(k, v) })
	return err
}

func (b *boundedSecrets) Delete(k secrets.Key) error {
	_, err := b.do(k, "kite.secrets.delete", func() (string, error) { return "", b.next.Delete(k) })
	return err
}

func (b *boundedSecrets) Backend() string { return b.next.Backend() }

type FeedControl interface {
	Start(ctx context.Context, apiKey, accessToken string) error
	Stop()
}

type LimiterControl interface {
	PauseAll(reason string)
	Resume()
}

type TokenSink interface {
	SetAPIKey(key string)
	SetAccessToken(token string)
}

// tokenGenerations is implemented by a TokenSink that numbers the tokens it installs (*Client.TokenGen,
// Ruling 70). A sink without it gives generation 0 (unknown), and token errors then end the session
// whatever generation they carry, as before Ruling 70.
type tokenGenerations interface {
	TokenGen() uint64
}

// installedTokenGen returns the generation of the token the sink holds now, or 0 when it does not number
// them. Callers read it right after SetAccessToken while holding exMu or feedMu, the locks every token
// install by the Session runs under, so no other install can come in between.
func (s *Session) installedTokenGen() uint64 {
	if g, ok := s.d.Tokens.(tokenGenerations); ok {
		return g.TokenGen()
	}
	return 0
}

type ProfileReader interface {
	Profile(ctx context.Context) (kiteconnect.UserProfile, error)
}

type Prober interface {
	Probe(ctx context.Context, apiKey, accessToken string) DataPlan
}

type SessionDeps struct {
	Settings        *settings.Settings
	Secrets         secrets.Store
	Repo            *store.SessionsRepo
	Clock           clock.Clock
	Bus             *bus.Bus
	Auth            func(apiKey string) AuthAPI
	Profiles        ProfileReader
	Tokens          TokenSink
	Limiter         LimiterControl
	Feed            FeedControl
	Probe           Prober
	Log             *slog.Logger
	ListenerTimeout time.Duration
	NewID           func() string
}

// Session owns the Kite login, the token lifecycle and the capability probe (ARCH §5.2, FR-ACCT-005/006/007).
type Session struct {
	d SessionDeps

	mu     sync.Mutex
	st     SessionState
	rec    *TokenRecord
	prior  State // state to fall back to when a login attempt fails or times out
	lb     *loopback
	nonces map[string]nonceBinding // state nonces issued in this run (Ruling 57)
	done   map[string]bool         // request tokens already exchanged in this run (reload-safe)
	gen    int                     // login attempt generation
	// tokGen is the token generation (Client.TokenGen) of the current session's token, set with rec at
	// each commit; 0 = unknown. HandleTokenError ignores errors from any other generation (Ruling 70).
	tokGen uint64
	runCtx context.Context
	// cancelBase cancels the base run context made by NewSession (Ruling 58 (2)): login timers and
	// probes started before Run wait on it, and Close (also at the end of Run) cancels it.
	cancelBase context.CancelFunc

	exMu sync.Mutex // serialises token exchange and logout
	// feedMu makes "is this session still current? then start the feed" (afterReady), "commit a new
	// login" (exchange) and "end the session, stop the feed, delete the token" (expire, endSession,
	// Logout) mutually exclusive, so the feed can never start with a token that was just deleted
	// (Ruling 52) and an ending session can never wipe a login that committed meanwhile (Ruling 58).
	// Lock order: exMu, then feedMu, then mu. feedMu is never held across a Kite call.
	feedMu sync.Mutex
}

// nonceBinding ties a state nonce to the login attempt (gen) that issued it and, once that attempt's
// exchange succeeded, to the request token it exchanged (Ruling 57).
type nonceBinding struct {
	gen   int
	token string
}

func NewSession(d SessionDeps) *Session {
	if d.NewID == nil {
		d.NewID = func() string { return uuid.Must(uuid.NewV7()).String() }
	}
	if d.ListenerTimeout == 0 {
		d.ListenerTimeout = defaultListenerTimeout
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Secrets != nil {
		d.Secrets = newBoundedSecrets(d.Secrets)
	}
	base, cancel := context.WithCancel(context.Background())
	return &Session{
		d:          d,
		st:         SessionState{State: StateNoCredentials, DataPlan: PlanUnknown},
		prior:      StateLoggedOut,
		nonces:     map[string]nonceBinding{},
		done:       map[string]bool{},
		runCtx:     base,
		cancelBase: cancel,
	}
}

func (s *Session) State() SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st
}

func (s *Session) set(st SessionState) SessionState {
	s.mu.Lock()
	s.st = st
	s.mu.Unlock()
	bus.Publish(s.d.Bus, TopicSessionState, st)
	return st
}

func (s *Session) ctx() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runCtx
}

func (s *Session) apiKey() string { return settings.Get(s.d.Settings, SettingAPIKey) }

// secret reads the API secret. secrets.ErrNotFound means "not configured"; any other error means the
// store could not be read (locked, timed out, damaged) and is returned so it never reads as "no credentials".
func (s *Session) secret() (string, error) {
	v, err := s.d.Secrets.Get(secrets.KeyAPISecret)
	if errors.Is(err, secrets.ErrNotFound) {
		return "", nil
	}
	return v, err
}

// configured reports whether both credentials are present. It reads the secret store at most once and
// not at all when the API key is unset (the key is saved after the secret, so no key means no credentials).
func (s *Session) configured() (bool, error) {
	if s.apiKey() == "" {
		return false, nil
	}
	v, err := s.secret()
	if err != nil {
		return false, err
	}
	return v != "", nil
}

// storeFailed logs the cause of a secret-store failure; callers show only msgSecretStore.
func (s *Session) storeFailed(what string, err error) {
	s.d.Log.Warn("secret store failure", "op", what, "err", err)
}

// idle is the resting state when no valid token exists. It never reads the secret store itself: the
// caller passes the result of its one configured() call. A store error gives logged_out with
// CodeSecretStore, never no_credentials.
func idle(configured bool, storeErr error, code, msg string) SessionState {
	st := SessionState{State: StateLoggedOut, DataPlan: PlanUnknown, ErrorCode: code, ErrorMessage: msg}
	switch {
	case storeErr != nil:
		st.ErrorCode, st.ErrorMessage = CodeSecretStore, msgSecretStore
	case !configured:
		st.State = StateNoCredentials
	}
	return st
}

func readyState(rec TokenRecord, plan DataPlan, ackRequired, offline bool) SessionState {
	return SessionState{State: StateReady, SessionID: rec.SessionID, UserID: rec.UserID, LoggedInAtMs: rec.LoggedInAtMs,
		ExpiresAtMs: rec.ExpiresAtMs, DataPlan: plan, RiskAckRequired: ackRequired, Offline: offline}
}

// CredentialsInfo describes the saved credentials. When the secret store cannot be read, Configured
// follows the saved API key (SaveCredentials stores the secret first), so an unreadable store does
// not show as "no credentials"; the session state carries CodeSecretStore.
func (s *Session) CredentialsInfo() CredentialsInfo {
	port := settings.Get(s.d.Settings, SettingRedirectPort)
	configured, err := s.configured()
	if err != nil {
		s.storeFailed("credentials info", err)
		configured = s.apiKey() != ""
	}
	return CredentialsInfo{Configured: configured, APIKeyMasked: MaskKey(s.apiKey()), Port: port,
		RedirectURL: RedirectURL(port), SecretBackend: s.d.Secrets.Backend()}
}

// SaveCredentials validates and stores the API key (settings) and API secret (secret store) (FR-ACCT-005).
func (s *Session) SaveCredentials(ctx context.Context, apiKey, apiSecret string) (SessionState, error) {
	k, sec := NormalizeCredential(apiKey), NormalizeCredential(apiSecret)
	if err := ValidateAPIKey(k); err != nil {
		return s.State(), err
	}
	if err := ValidateAPISecret(sec); err != nil {
		return s.State(), err
	}
	if k == sec {
		return s.State(), &LoginError{Code: CodeInvalidCredentials, Message: "The API key and the API secret are identical. Paste each from its own field in the Kite developer console."}
	}
	switch s.State().State {
	case StateLoggingIn:
		return s.State(), &LoginError{Code: CodeLoginBusy, Message: "Finish or cancel the current login before changing credentials."}
	case StateReady:
		return s.State(), &LoginError{Code: CodeAlreadyLoggedIn, Message: "Log out before changing the Kite credentials."}
	}
	logging.RegisterSecret(sec)
	if err := s.d.Secrets.Set(secrets.KeyAPISecret, sec); err != nil {
		s.storeFailed("save api secret", err)
		return s.State(), &LoginError{Code: CodeSecretStore, Message: msgSecretStore}
	}
	if err := settings.Set(ctx, s.d.Settings, SettingAPIKey, k); err != nil {
		return s.State(), err
	}
	s.d.Tokens.SetAPIKey(k)
	s.d.Log.Info("kite credentials saved", "api_key", MaskKey(k), "secret_backend", s.d.Secrets.Backend())
	return s.set(idle(true, nil, "", "")), nil
}

// Restore resumes a still-valid session at startup without asking for a login (FR-ACCT-006 AC4).
//
// Ruling 59 (1): Restore holds exMu for its whole run (its Kite call included), so a Logout or a paste
// login waits for it and then wins. Its commit (s.rec → ready) runs under feedMu and is abandoned when a
// login attempt began meanwhile (gen changed) or another session is current; the paste login that is
// waiting on exMu then commits on its own. Ruling 59 (2): a stored token whose session row already ended
// (a logout that could not delete it) is deleted, never resumed, and Kite is not called.
func (s *Session) Restore(ctx context.Context) SessionState {
	s.exMu.Lock()
	defer s.exMu.Unlock()
	s.mu.Lock()
	gen := s.gen
	s.mu.Unlock()

	configured, err := s.configured()
	if err != nil {
		s.storeFailed("read api secret", err)
		s.d.Limiter.PauseAll("no credentials")
		return s.restoreIdle(gen, idle(false, err, "", ""))
	}
	if !configured {
		s.d.Limiter.PauseAll("no credentials")
		return s.set(idle(false, nil, "", ""))
	}
	apiKey := s.apiKey()
	s.d.Tokens.SetAPIKey(apiKey)
	raw, err := s.d.Secrets.Get(secrets.KeyAccessToken)
	if err != nil {
		s.d.Limiter.PauseAll("logged out")
		if errors.Is(err, secrets.ErrNotFound) {
			return s.restoreIdle(gen, idle(true, nil, "", ""))
		}
		s.storeFailed("read access token", err)
		return s.restoreIdle(gen, idle(true, err, "", ""))
	}
	var rec TokenRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil || rec.AccessToken == "" || rec.SessionID == "" {
		if derr := s.d.Secrets.Delete(secrets.KeyAccessToken); derr != nil {
			s.storeFailed("delete unreadable access token", derr)
		}
		s.d.Limiter.PauseAll("logged out")
		return s.restoreIdle(gen, idle(true, nil, "", ""))
	}
	logging.RegisterSecret(rec.AccessToken)
	row, rowErr := s.d.Repo.Get(ctx, rec.SessionID)
	if rowErr != nil && !errors.Is(rowErr, store.ErrNoRow) {
		// Ruling 63 (3): without the row we cannot tell whether this session was logged out; fail closed.
		// The token is kept (the read error may be transient); a new login replaces it.
		s.d.Log.Warn("restore: reading the session row failed; not resuming", "session_id", rec.SessionID, "err", rowErr)
		s.d.Limiter.PauseAll("logged out")
		return s.restoreIdle(gen, idle(true, nil, "", ""))
	}
	if rowErr == nil && row.EndedAtMs != 0 {
		s.d.Log.Warn("restore: the stored token belongs to an ended session; deleting it", "session_id", rec.SessionID, "end_reason", row.EndReason)
		s.d.Limiter.PauseAll("logged out")
		derr := s.deleteToken()
		if derr != nil {
			s.storeFailed("delete ended session's access token", derr)
		}
		if row.EndReason == endReasonLogout && s.d.Clock.Now().UnixMilli() < rec.ExpiresAtMs {
			s.invalidateEnded(apiKey, rec)
		}
		return s.restoreIdle(gen, idle(true, derr, "", ""))
	}
	if s.d.Clock.Now().UnixMilli() >= rec.ExpiresAtMs {
		return s.endSessionAtRestore(gen, rec, CodeSessionExpired)
	}
	s.d.Tokens.SetAccessToken(rec.AccessToken)
	tokGen := s.installedTokenGen()
	s.d.Limiter.Resume()
	offline := false
	if _, err := s.d.Profiles.Profile(ctx); err != nil {
		if isTokenError(err) {
			return s.endSessionAtRestore(gen, rec, CodeTokenRejected)
		}
		offline = true
		s.d.Log.Warn("Kite unreachable at startup; continuing with cached data", "kind", errKind(err))
	}
	plan, ackRequired := PlanUnknown, true
	switch {
	case rowErr == nil:
		if row.DataPlan != "" {
			plan = DataPlan(row.DataPlan)
		}
		ackRequired = row.RiskAckAtMs == 0
	default: // store.ErrNoRow; any other read error returned above (Ruling 63 (3))
		// The token outlived its row (e.g. app.db restored from a backup): recreate it so the risk
		// acknowledgement and the data plan of this session can be persisted.
		s.d.Log.Warn("restore: session row missing; recreating it", "session_id", rec.SessionID)
		if ierr := s.insertRow(ctx, rec, plan); ierr != nil {
			s.d.Log.Warn("recreating the session row failed", "session_id", rec.SessionID, "err", ierr)
		}
	}
	s.feedMu.Lock()
	s.mu.Lock()
	if s.gen != gen || s.supersededLocked(rec.SessionID) || s.st.State == StateLoggingIn {
		// A login began during Restore: it owns the session state now.
		cur, corrected := s.correctNoCredentialsLocked()
		s.mu.Unlock()
		s.d.Tokens.SetAccessToken("")
		s.d.Limiter.PauseAll("logged out")
		s.feedMu.Unlock()
		if corrected {
			bus.Publish(s.d.Bus, TopicSessionState, cur)
		}
		s.d.Log.Info("restore abandoned: a login began meanwhile", "session_id", rec.SessionID)
		return cur
	}
	st := readyState(rec, plan, ackRequired, offline)
	s.rec = &rec
	s.tokGen = tokGen
	s.st = st
	s.mu.Unlock()
	s.feedMu.Unlock()
	bus.Publish(s.d.Bus, TopicSessionState, st)
	s.afterReady(apiKey, rec)
	return st
}

// endReasonLogout is the session-row end reason Logout records.
const endReasonLogout = "logout"

// correctNoCredentialsLocked (mu held) fixes the state after Restore saw configured credentials while a
// login attempt began meanwhile: BeginLogin's fallback may be the initial no_credentials state, which is
// wrong once credentials are known to exist (Ruling 63 (1)). It reports whether the visible state changed.
func (s *Session) correctNoCredentialsLocked() (SessionState, bool) {
	if s.prior == StateNoCredentials {
		s.prior = StateLoggedOut // a cancelled or failed login falls back to logged_out
	}
	if s.st.State != StateNoCredentials {
		return s.st, false
	}
	s.st.State = StateLoggedOut
	return s.st, true
}

// restoreIdle commits Restore's idle result st only while no login attempt began since Restore started
// (gen unchanged); otherwise that attempt owns the state and Restore only corrects a stale
// no_credentials fallback (Ruling 63 (2)). Restore's callers know the credentials are configured.
func (s *Session) restoreIdle(gen int, st SessionState) SessionState {
	s.mu.Lock()
	if s.gen == gen {
		s.st = st
		s.mu.Unlock()
		bus.Publish(s.d.Bus, TopicSessionState, st)
		return st
	}
	cur, corrected := s.correctNoCredentialsLocked()
	s.mu.Unlock()
	if corrected {
		bus.Publish(s.d.Bus, TopicSessionState, cur)
	}
	return cur
}

// endSessionAtRestore ends rec's session at startup (expired or rejected token). When a login attempt
// began since Restore started (gen changed), that attempt owns the state: the token is still deleted, the
// traffic stopped and the row ended, but the state is left alone apart from the no_credentials correction
// (Ruling 68 (2)). A login cannot commit meanwhile: its exchange waits for exMu, which Restore holds.
func (s *Session) endSessionAtRestore(gen int, rec TokenRecord, code string) SessionState {
	s.feedMu.Lock()
	defer s.feedMu.Unlock()
	s.mu.Lock()
	if s.gen == gen {
		s.mu.Unlock()
		return s.endSessionLocked(rec, code)
	}
	cur, corrected := s.correctNoCredentialsLocked()
	s.mu.Unlock()
	s.stopTraffic("session expired")
	if err := s.deleteToken(); err != nil {
		s.storeFailed("delete access token", err)
	}
	s.endRow(rec.SessionID, code)
	s.d.Log.Info("kite session ended at startup; a login in progress keeps the state", "reason", code)
	if corrected {
		bus.Publish(s.d.Bus, TopicSessionState, cur)
	}
	return cur
}

// invalidateEnded asks Kite to invalidate the unexpired token of a session that was logged out but whose
// token survived (Ruling 63 (4)). It is best effort, runs off the startup path on the run context and is
// bounded by endedInvalidateTimeout (the Kite client ignores ctx, so the wait is bounded, not the call).
// The outcome is logged at Warn with the error kind only; the token never reaches a log line.
func (s *Session) invalidateEnded(apiKey string, rec TokenRecord) {
	ctx, cancel := context.WithTimeout(s.ctx(), endedInvalidateTimeout)
	auth := s.d.Auth(apiKey)
	logging.Go("kite.restore.invalidate", func() {
		defer cancel()
		res := make(chan error, 1)
		logging.Go("kite.restore.invalidate.call", func() { res <- auth.Invalidate(ctx, rec.AccessToken) })
		var err error
		kind := ""
		select {
		case err = <-res:
			if err != nil {
				kind = errKind(err)
			}
		case <-ctx.Done():
			err = ctx.Err()
			kind = "canceled"
			if errors.Is(err, context.DeadlineExceeded) {
				kind = "timeout"
			}
		}
		s.d.Log.Warn("restore: invalidating the ended session's token", "session_id", rec.SessionID,
			"invalidated", err == nil, "kind", kind)
	})
}

// AckRisk records the FR-SHELL-020 acknowledgement for the current session id.
func (s *Session) AckRisk(ctx context.Context) (SessionState, error) {
	st := s.State()
	if st.State != StateReady {
		return st, &LoginError{Code: CodeNotLoggedIn, Message: "Log in to Kite first."}
	}
	at := s.d.Clock.Now().UnixMilli()
	err := s.d.Repo.AckRisk(ctx, st.SessionID, at)
	if errors.Is(err, store.ErrNoRow) {
		// No row to record against (Ruling 37): recreate it from the current session, then record.
		// Re-check first: a session that ended meanwhile must not get its row back.
		if !s.isCurrent(st.SessionID) {
			return s.State(), &LoginError{Code: CodeNotLoggedIn, Message: "Log in to Kite first."}
		}
		s.d.Log.Warn("risk acknowledgement: session row missing; recreating it", "session_id", st.SessionID)
		rec := TokenRecord{SessionID: st.SessionID, UserID: st.UserID, LoggedInAtMs: st.LoggedInAtMs, ExpiresAtMs: st.ExpiresAtMs}
		if err = s.insertRow(ctx, rec, st.DataPlan); err == nil {
			err = s.d.Repo.AckRisk(ctx, st.SessionID, at)
		}
	}
	if err != nil {
		// Never report an acknowledgement that was not persisted (FR-SHELL-020).
		return st, fmt.Errorf("kite: recording the risk acknowledgement failed: %w", err)
	}
	s.mu.Lock()
	if s.st.SessionID == st.SessionID {
		s.st.RiskAckRequired = false
	}
	st = s.st
	s.mu.Unlock()
	bus.Publish(s.d.Bus, TopicSessionState, st)
	return st, nil
}

// afterReady runs the capability probe and starts the ticker unless the key has no data plan (FR-ACCT-007).
func (s *Session) afterReady(apiKey string, rec TokenRecord) {
	ctx := s.ctx()
	logging.Go("kite.session.probe", func() {
		plan := PlanUnknown
		for attempt := 0; attempt < probeAttempts && plan == PlanUnknown; attempt++ {
			if attempt > 0 {
				retry := s.d.Clock.At(s.d.Clock.Now().Add(probeRetry))
				select {
				case <-ctx.Done():
					retry.Stop()
					return
				case <-retry.C():
				}
			}
			if !s.isCurrent(rec.SessionID) {
				return
			}
			plan = s.d.Probe.Probe(ctx, apiKey, rec.AccessToken)
		}
		s.feedMu.Lock()
		defer s.feedMu.Unlock()
		if !s.applyPlan(rec.SessionID, plan) {
			return
		}
		if plan == PlanNone {
			s.d.Log.Info("API key has no market-data permission; running in orders-and-portfolio-only mode")
			return
		}
		// Still under feedMu: endSession cannot run between the isCurrent check in applyPlan and here.
		if err := s.d.Feed.Start(ctx, apiKey, rec.AccessToken); err != nil {
			s.d.Log.Warn("ticker start failed", "err", err)
		}
	})
}

// insertRow writes the sessions row for rec; a row that already exists counts as success.
func (s *Session) insertRow(ctx context.Context, rec TokenRecord, plan DataPlan) error {
	err := s.d.Repo.Insert(ctx, store.SessionRow{ID: rec.SessionID, UserID: rec.UserID, LoggedInAtMs: rec.LoggedInAtMs,
		ExpiresAtMs: rec.ExpiresAtMs, DataPlan: string(plan)})
	if errors.Is(err, store.ErrDuplicate) {
		return nil
	}
	return err
}

func (s *Session) isCurrent(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.State == StateReady && s.st.SessionID == sessionID
}

func (s *Session) applyPlan(sessionID string, plan DataPlan) bool {
	s.mu.Lock()
	if s.st.State != StateReady || s.st.SessionID != sessionID {
		s.mu.Unlock()
		return false
	}
	s.st.DataPlan = plan
	st := s.st
	s.mu.Unlock()
	if err := s.d.Repo.SetDataPlan(context.Background(), sessionID, string(plan)); errors.Is(err, store.ErrNoRow) {
		s.d.Log.Warn("recording the data plan failed: no session row", "session_id", sessionID)
	} else if err != nil {
		s.d.Log.Warn("recording the data plan failed", "err", err)
	}
	bus.Publish(s.d.Bus, TopicSessionState, st)
	return true
}

// endSession performs the side effects of a session ending through expiry or rejection.
func (s *Session) endSession(rec TokenRecord, code string) SessionState {
	s.feedMu.Lock()
	defer s.feedMu.Unlock()
	return s.endSessionLocked(rec, code)
}

// endSessionLocked runs with feedMu held. It first leaves the ready state (so a probe still in flight
// finds the session no longer current), then stops the feed and deletes the token; a Feed.Start already
// running finishes first and is stopped. Ruling 58 (1): when a newer session committed while this call
// waited for feedMu, only rec's row is ended; the new session's state, feed and token are left alone.
func (s *Session) endSessionLocked(rec TokenRecord, code string) SessionState {
	st := SessionState{State: StateExpired, UserID: rec.UserID, DataPlan: PlanUnknown, ErrorCode: code, ErrorMessage: msgExpired}
	s.mu.Lock()
	if s.supersededLocked(rec.SessionID) {
		cur := s.st
		s.mu.Unlock()
		s.endRow(rec.SessionID, code)
		s.d.Log.Info("kite session ended after a newer login; the new session is kept", "reason", code)
		return cur
	}
	s.st = st
	s.rec = nil
	s.mu.Unlock()
	s.stopTraffic("session expired")
	if err := s.deleteToken(); err != nil {
		s.storeFailed("delete access token", err)
	}
	s.endRow(rec.SessionID, code)
	s.d.Log.Info("kite session ended", "reason", code)
	bus.Publish(s.d.Bus, TopicSessionState, st)
	return st
}

// supersededLocked reports (mu held) whether a session other than sessionID is the current one.
func (s *Session) supersededLocked(sessionID string) bool {
	return (s.rec != nil && s.rec.SessionID != sessionID) || (s.st.State == StateReady && s.st.SessionID != sessionID)
}

// stopTraffic stops the ticker, pauses all Kite REST traffic and clears the in-memory token. Callers
// hold feedMu, so a probe cannot start the feed again in between.
func (s *Session) stopTraffic(reason string) {
	s.d.Feed.Stop()
	s.d.Limiter.PauseAll(reason)
	s.d.Tokens.SetAccessToken("")
}

// deleteToken removes the stored token record; an absent record is not an error.
func (s *Session) deleteToken() error {
	if err := s.d.Secrets.Delete(secrets.KeyAccessToken); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		return err
	}
	return nil
}

func (s *Session) endRow(sessionID, reason string) {
	err := s.d.Repo.End(context.Background(), sessionID, s.d.Clock.Now().UnixMilli(), reason)
	switch {
	case errors.Is(err, store.ErrNoRow):
		s.d.Log.Info("no session row to end", "session_id", sessionID, "reason", reason)
	case err != nil:
		s.d.Log.Warn("recording the session end failed", "err", err)
	}
}

func isTokenError(err error) bool {
	var ke kiteconnect.Error
	if errors.As(err, &ke) {
		return ke.ErrorType == kiteconnect.TokenError
	}
	return Classify(err) == KindSession
}

// errKind is a log-safe description of an error (never the message of a transport error, which may hold a URL).
func errKind(err error) string {
	var ke kiteconnect.Error
	if errors.As(err, &ke) {
		return ke.ErrorType
	}
	return "transport"
}
