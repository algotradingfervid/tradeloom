// internal/service/session.go
package service

import (
	"context"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite"
	"tradeloom/internal/logging"
)

type (
	CredentialsInfo = kite.CredentialsInfo
	LoginStart      = kite.LoginStart
	LogoutResult    = kite.LogoutResult
	Profile         = kite.Profile
)

type CredentialsInput struct {
	APIKey    string `json:"apiKey"`
	APISecret string `json:"apiSecret"`
}

// SessionStatus is the payload of Session.Status() and of the "session.state" event.
type SessionStatus struct {
	State           string            `json:"state"`
	SessionID       string            `json:"sessionId"`
	UserID          string            `json:"userId"`
	LoggedInAtMs    int64             `json:"loggedInAtMs"`
	ExpiresAtMs     int64             `json:"expiresAtMs"`
	DataPlan        string            `json:"dataPlan"`
	Offline         bool              `json:"offline"`
	RiskAckRequired bool              `json:"riskAckRequired"`
	ErrorCode       string            `json:"errorCode"`
	ErrorMessage    string            `json:"errorMessage"`
	Credentials     CredentialsInfo   `json:"credentials"`
	Secrets         SecretStorageInfo `json:"secrets"`
}

// StateLocked is reported while an encrypted secrets file exists but has not been unlocked this start.
const StateLocked = "locked"

func (s *Service) statusFrom(st kite.SessionState) SessionStatus {
	out := SessionStatus{
		State: string(st.State), SessionID: st.SessionID, UserID: st.UserID, LoggedInAtMs: st.LoggedInAtMs,
		ExpiresAtMs: st.ExpiresAtMs, DataPlan: string(st.DataPlan), Offline: st.Offline, RiskAckRequired: st.RiskAckRequired,
		ErrorCode: st.ErrorCode, ErrorMessage: st.ErrorMessage, Credentials: s.d.Session.CredentialsInfo(), Secrets: s.secretInfo(),
	}
	if s.d.Secrets != nil && s.d.Secrets.Locked() {
		out.State = StateLocked
	}
	return out
}

func (s *Service) SessionStatus(_ context.Context) SessionStatus {
	return s.statusFrom(s.d.Session.State())
}

func (s *Service) CredentialsInfo(_ context.Context) CredentialsInfo {
	return s.d.Session.CredentialsInfo()
}

func (s *Service) SaveCredentials(ctx context.Context, in CredentialsInput) (SessionStatus, error) {
	st, err := s.d.Session.SaveCredentials(ctx, in.APIKey, in.APISecret)
	return s.statusFrom(st), s.refuse("credentials", err)
}

func (s *Service) RemoveCredentials(ctx context.Context) (SessionStatus, error) {
	st, err := s.d.Session.RemoveCredentials(ctx)
	return s.statusFrom(st), s.refuse("credentials", err)
}

func (s *Service) SetRedirectPort(_ context.Context, port int) (CredentialsInfo, error) {
	info, err := s.d.Session.SetRedirectPort(port)
	return info, s.refuse("credentials", err)
}

func (s *Service) BeginLogin(ctx context.Context) (LoginStart, error) {
	ls, err := s.d.Session.BeginLogin(ctx)
	return ls, s.refuse("login", err)
}

func (s *Service) CompleteLogin(ctx context.Context, pasted string) (SessionStatus, error) {
	st, err := s.d.Session.CompleteLogin(ctx, pasted)
	return s.statusFrom(st), s.refuse("login", err)
}

func (s *Service) CancelLogin(_ context.Context) SessionStatus {
	return s.statusFrom(s.d.Session.CancelLogin())
}

func (s *Service) Logout(ctx context.Context) (LogoutResult, error) {
	r, err := s.d.Session.Logout(ctx)
	return r, s.refuse("logout", err)
}

func (s *Service) AckRiskDisclosure(ctx context.Context) (SessionStatus, error) {
	st, err := s.d.Session.AckRisk(ctx)
	return s.statusFrom(st), s.refuse("risk", err)
}

func (s *Service) Profile(ctx context.Context) (Profile, error) {
	p, err := s.d.Session.Profile(ctx)
	return p, s.refuse("profile", err)
}

// Restore resumes the session at startup, unless the secrets file is still locked (Unlock restores later).
func (s *Service) Restore(ctx context.Context) SessionStatus {
	if s.d.Secrets != nil && s.d.Secrets.Locked() {
		return s.SessionStatus(ctx)
	}
	return s.statusFrom(s.d.Session.Restore(ctx))
}

// RunSession runs the session watcher (06:00 expiry, login timers). The caller must start it before the UI
// can call BeginLogin, so login timers observe shutdown (Task 8 carry-forward).
func (s *Service) RunSession(ctx context.Context) { s.d.Session.Run(ctx) }

// Run supervises session-scoped services: orders.Sync runs only while the session is ready.
// A bus event is only a wake-up: Run always applies Session.State(), because publishes from kite.Session
// can be delivered out of order and a stale "ready" must not restart Sync after a logout (Ruling 61).
// stop() waits for the previous Orders.Run to return, so two runs never overlap.
func (s *Service) Run(ctx context.Context) {
	sub := bus.Subscribe(s.d.Bus, kite.TopicSessionState, bus.SubOpts[kite.SessionState]{Name: "service.supervisor", Buffer: 16})
	defer bus.Unsubscribe(s.d.Bus, sub)
	var (
		cancel  context.CancelFunc
		done    chan struct{}
		session string
	)
	stop := func() {
		if cancel == nil {
			return
		}
		cancel()
		<-done
		cancel, done, session = nil, nil, ""
	}
	apply := func() {
		st := s.d.Session.State()
		if st.State != kite.StateReady {
			stop()
			return
		}
		s.d.Orders.SetDataPlan(st.DataPlan)
		if cancel != nil && session == st.SessionID {
			return
		}
		stop()
		c, cn := context.WithCancel(ctx)
		d := make(chan struct{})
		cancel, done, session = cn, d, st.SessionID
		logging.Go("orders.sync", func() {
			defer close(d)
			s.d.Orders.Run(c)
		})
	}
	defer stop()
	apply()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-sub.C():
			if !ok {
				return
			}
			apply()
		}
	}
}
