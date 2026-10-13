// internal/service/servicetest/fakes.go
// Package servicetest builds a *service.Service with a fake session for tests of internal/app.
package servicetest

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/orders"
	"tradeloom/internal/secrets"
	"tradeloom/internal/service"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

type FakeSession struct {
	mu        sync.Mutex
	St        kite.SessionState
	Err       error
	LoginURL  string
	RunBlock  chan struct{} // non-nil: Run keeps running after ctx is cancelled until RunBlock is closed (slow-exit tests)
	completed []string
	forwarded []string
}

func (f *FakeSession) Set(st kite.SessionState) { f.mu.Lock(); defer f.mu.Unlock(); f.St = st }
func (f *FakeSession) State() kite.SessionState { f.mu.Lock(); defer f.mu.Unlock(); return f.St }
func (f *FakeSession) Completed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.completed...)
}

// Forwarded lists the redirect URLs handed over by a second launch (CompleteForwardedLogin, Ruling 57).
func (f *FakeSession) Forwarded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.forwarded...)
}
func (f *FakeSession) CompleteForwardedLogin(_ context.Context, u string) (kite.SessionState, error) {
	f.mu.Lock()
	f.forwarded = append(f.forwarded, u)
	f.mu.Unlock()
	return f.State(), f.Err
}
func (f *FakeSession) CredentialsInfo() kite.CredentialsInfo {
	return kite.CredentialsInfo{Port: 7420, RedirectURL: kite.RedirectURL(7420), SecretBackend: "memory"}
}
func (f *FakeSession) SaveCredentials(context.Context, string, string) (kite.SessionState, error) {
	return f.State(), f.Err
}
func (f *FakeSession) RemoveCredentials(context.Context) (kite.SessionState, error) {
	return f.State(), f.Err
}
func (f *FakeSession) SetRedirectPort(int) (kite.CredentialsInfo, error) {
	return f.CredentialsInfo(), f.Err
}
func (f *FakeSession) BeginLogin(context.Context) (kite.LoginStart, error) {
	return kite.LoginStart{LoginURL: f.LoginURL, RedirectURL: kite.RedirectURL(7420), ListenerOK: true}, f.Err
}
func (f *FakeSession) CompleteLogin(_ context.Context, p string) (kite.SessionState, error) {
	f.mu.Lock()
	f.completed = append(f.completed, p)
	f.mu.Unlock()
	return f.State(), f.Err
}
func (f *FakeSession) CancelLogin() kite.SessionState { return f.State() }
func (f *FakeSession) Logout(context.Context) (kite.LogoutResult, error) {
	return kite.LogoutResult{}, f.Err
}
func (f *FakeSession) AckRisk(context.Context) (kite.SessionState, error) { return f.State(), f.Err }
func (f *FakeSession) Profile(context.Context) (kite.Profile, error)      { return kite.Profile{}, f.Err }
func (f *FakeSession) Restore(context.Context) kite.SessionState          { return f.State() }
func (f *FakeSession) Run(ctx context.Context) {
	<-ctx.Done()
	if f.RunBlock != nil {
		<-f.RunBlock
	}
}

type noSync struct{}

func (noSync) Run(ctx context.Context)                     { <-ctx.Done() }
func (noSync) SetDataPlan(kite.DataPlan)                   {}
func (noSync) Refresh(context.Context, orders.Scope) error { return nil }
func (noSync) Book() orders.BookSnapshot                   { return orders.BookSnapshot{} }
func (noSync) Positions() orders.PositionsSnapshot         { return orders.PositionsSnapshot{} }
func (noSync) Holdings() orders.HoldingsSnapshot           { return orders.HoldingsSnapshot{} }

// New returns a Service over real settings/watchlists/shortcuts/UI-state storage in a temporary data
// directory, the shipped calendar, a "no storage" secrets switch and a fake session. Chart, quote,
// candle-store, refresher and feed dependencies are nil (the app tests never reach them).
func New(t testing.TB) (*service.Service, *FakeSession, *bus.Bus) {
	t.Helper()
	db := storetest.Open(t)
	b := bus.New(nil)
	t.Cleanup(b.Close)
	st, err := settings.Load(context.Background(), db.SettingsRepo(), b)
	if err != nil {
		t.Fatal(err)
	}
	cal, err := calendar.NewDefault()
	if err != nil {
		t.Fatal(err)
	}
	layout := store.NewLayout(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	fs := &FakeSession{St: kite.SessionState{State: kite.StateLoggedOut}, LoginURL: "https://kite.zerodha.com/connect/login?api_key=k&v=3"}
	svc := service.New(service.Deps{
		Session: fs, Orders: noSync{}, Settings: st, Watchlists: store.NewWatchlistsRepo(db), Calendar: cal,
		Secrets: service.NewSecretSwitch(secrets.NewNone(), false), SecretsFile: layout.SecretsFile,
		Detection: secrets.Detection{Mode: secrets.ModeChoose, Choices: []secrets.Choice{secrets.ChoiceEncryptedFile, secrets.ChoiceNoStorage}},
		Shortcuts: store.NewShortcutsRepo(db), UiState: store.NewWindowStateRepo(db), DB: db, Layout: layout, AppVersion: "test",
		Clock: clock.NewFake(time.Date(2026, 10, 12, 9, 0, 0, 0, clock.IST)), Bus: b, Log: slog.Default(),
		NewID: func() string { return "id-1" },
	})
	return svc, fs, b
}
