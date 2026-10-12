// internal/app/api/session.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

// Session is bound to Wails as window.go.api.Session (frontend/wailsjs/go/api/Session).
type Session struct {
	svc  *service.Service
	ctx  func() context.Context
	open func(url string)
}

func NewSession(svc *service.Service, ctx func() context.Context, open func(string)) *Session {
	return &Session{svc: svc, ctx: ctx, open: open}
}

func (s *Session) Status() service.SessionStatus         { return s.svc.SessionStatus(s.ctx()) }
func (s *Session) Credentials() service.CredentialsInfo  { return s.svc.CredentialsInfo(s.ctx()) }
func (s *Session) CancelLogin() service.SessionStatus    { return s.svc.CancelLogin(s.ctx()) }
func (s *Session) Profile() (service.Profile, error)     { return s.svc.Profile(s.ctx()) }
func (s *Session) Logout() (service.LogoutResult, error) { return s.svc.Logout(s.ctx()) }
func (s *Session) RemoveCredentials() (service.SessionStatus, error) {
	return s.svc.RemoveCredentials(s.ctx())
}
func (s *Session) AckRiskDisclosure() (service.SessionStatus, error) {
	return s.svc.AckRiskDisclosure(s.ctx())
}

func (s *Session) SaveCredentials(in service.CredentialsInput) (service.SessionStatus, error) {
	return s.svc.SaveCredentials(s.ctx(), in)
}

func (s *Session) SetRedirectPort(port int) (service.CredentialsInfo, error) {
	return s.svc.SetRedirectPort(s.ctx(), port)
}

func (s *Session) CompleteLogin(pasted string) (service.SessionStatus, error) {
	return s.svc.CompleteLogin(s.ctx(), pasted)
}

// Login starts a login attempt and opens Kite's login page in the system browser (FR-ACCT-006).
func (s *Session) Login() (service.LoginStart, error) {
	ls, err := s.svc.BeginLogin(s.ctx())
	if err != nil {
		return service.LoginStart{}, err
	}
	s.open(ls.LoginURL)
	return ls, nil
}

// Secret storage (FR-ACCT-013, Task 27).
func (s *Session) SecretStorage() service.SecretStorageInfo { return s.svc.SecretStorage(s.ctx()) }
func (s *Session) UseEncryptedFile(passphrase string) (service.SessionStatus, error) {
	return s.svc.UseEncryptedFile(s.ctx(), passphrase)
}
func (s *Session) RefuseSecretStorage() (service.SessionStatus, error) {
	return s.svc.RefuseSecretStorage(s.ctx())
}
func (s *Session) Unlock(passphrase string) (service.SessionStatus, error) {
	return s.svc.Unlock(s.ctx(), passphrase)
}
func (s *Session) MigrateToKeyring() (service.SecretStorageInfo, error) {
	return s.svc.MigrateToKeyring(s.ctx())
}

// ReconnectFeed is the status indicator's "Retry now".
func (s *Session) ReconnectFeed() error { return s.svc.ReconnectFeed(s.ctx()) }
