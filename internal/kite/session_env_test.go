// internal/kite/session_env_test.go
package kite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/bus"
	"tradeloom/internal/clock"
	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

const (
	testKey    = "kitekey1234abcd"
	testSecret = "s3cr3tvalue0987654321zyxwvutsrq"
)

type memSecrets struct {
	mu sync.Mutex
	m  map[secrets.Key]string
}

func (s *memSecrets) Get(k secrets.Key) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[k]
	if !ok {
		return "", secrets.ErrNotFound
	}
	return v, nil
}
func (s *memSecrets) Set(k secrets.Key, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}
func (s *memSecrets) Delete(k secrets.Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[k]; !ok {
		return secrets.ErrNotFound
	}
	delete(s.m, k)
	return nil
}
func (s *memSecrets) Backend() string { return "memory" }
func (s *memSecrets) has(k secrets.Key) bool {
	_, err := s.Get(k)
	return err == nil
}

type fakeAuth struct {
	mu          sync.Mutex
	valid       map[string]AuthSession
	used        map[string]bool
	exchanges   int
	invalidated []string
	invErr      error
}

func (f *fakeAuth) allow(rt string, s AuthSession) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.valid[rt] = s
}
func (f *fakeAuth) LoginURL(state string) string {
	return "https://kite.zerodha.com/connect/login?api_key=" + testKey + "&v=3&redirect_params=" + url.QueryEscape("state="+state)
}
func (f *fakeAuth) GenerateSession(_ context.Context, rt, _ string) (AuthSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.exchanges++
	s, ok := f.valid[rt]
	if !ok || f.used[rt] {
		return AuthSession{}, APIError(kiteconnect.TokenError, 403, "Token is invalid or has expired.")
	}
	f.used[rt] = true
	return s, nil
}
func (f *fakeAuth) Invalidate(_ context.Context, tok string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.invalidated = append(f.invalidated, tok)
	return f.invErr
}
func (f *fakeAuth) exchangeCount() int { f.mu.Lock(); defer f.mu.Unlock(); return f.exchanges }

type fakeFeed struct {
	mu            sync.Mutex
	starts, stops int
	token         string
}

func (f *fakeFeed) Start(_ context.Context, _, tok string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	f.token = tok
	return nil
}
func (f *fakeFeed) Stop()           { f.mu.Lock(); defer f.mu.Unlock(); f.stops++ }
func (f *fakeFeed) startCount() int { f.mu.Lock(); defer f.mu.Unlock(); return f.starts }
func (f *fakeFeed) stopCount() int  { f.mu.Lock(); defer f.mu.Unlock(); return f.stops }

type fakeLimiter struct {
	mu              sync.Mutex
	paused          bool
	pauses, resumes int
}

func (f *fakeLimiter) PauseAll(string) { f.mu.Lock(); defer f.mu.Unlock(); f.paused = true; f.pauses++ }
func (f *fakeLimiter) Resume()         { f.mu.Lock(); defer f.mu.Unlock(); f.paused = false; f.resumes++ }
func (f *fakeLimiter) isPaused() bool  { f.mu.Lock(); defer f.mu.Unlock(); return f.paused }
func (f *fakeLimiter) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pauses, f.resumes
}

type fakeTokens struct {
	mu         sync.Mutex
	key, token string
}

func (f *fakeTokens) SetAPIKey(k string)      { f.mu.Lock(); defer f.mu.Unlock(); f.key = k }
func (f *fakeTokens) SetAccessToken(t string) { f.mu.Lock(); defer f.mu.Unlock(); f.token = t }
func (f *fakeTokens) get() string             { f.mu.Lock(); defer f.mu.Unlock(); return f.token }

type fakeProfiles struct {
	mu    sync.Mutex
	err   error
	p     kiteconnect.UserProfile
	calls int
}

func (f *fakeProfiles) Profile(context.Context) (kiteconnect.UserProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.p, f.err
}
func (f *fakeProfiles) setErr(err error) { f.mu.Lock(); defer f.mu.Unlock(); f.err = err }
func (f *fakeProfiles) callCount() int   { f.mu.Lock(); defer f.mu.Unlock(); return f.calls }

type fakeProbe struct {
	mu   sync.Mutex
	plan DataPlan
}

func (f *fakeProbe) Probe(context.Context, string, string) DataPlan {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.plan
}
func (f *fakeProbe) set(p DataPlan) { f.mu.Lock(); defer f.mu.Unlock(); f.plan = p }

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

type testEnv struct {
	t        *testing.T
	sess     *Session
	secrets  *memSecrets
	auth     *fakeAuth
	feed     *fakeFeed
	lim      *fakeLimiter
	tokens   *fakeTokens
	profiles *fakeProfiles
	probe    *fakeProbe
	clock    *clock.Fake
	settings *settings.Settings
	db       *store.DB
	repo     *store.SessionsRepo
	bus      *bus.Bus
	logs     *syncBuffer
}

func newEnv(t *testing.T, at time.Time) *testEnv {
	t.Helper()
	db := storetest.Open(t)
	b := bus.New(nil)
	t.Cleanup(b.Close)
	st, err := settings.Load(context.Background(), db.SettingsRepo(), b)
	if err != nil {
		t.Fatal(err)
	}
	e := &testEnv{
		t: t, db: db, bus: b, settings: st,
		secrets:  &memSecrets{m: map[secrets.Key]string{}},
		auth:     &fakeAuth{valid: map[string]AuthSession{}, used: map[string]bool{}},
		feed:     &fakeFeed{},
		lim:      &fakeLimiter{},
		tokens:   &fakeTokens{},
		profiles: &fakeProfiles{p: kiteconnect.UserProfile{UserID: "AB1234", UserName: "Asha Bhat", Email: "asha@example.com", Broker: "ZERODHA"}},
		probe:    &fakeProbe{plan: PlanConnect},
		clock:    clock.NewFake(at),
		repo:     store.NewSessionsRepo(db),
		logs:     &syncBuffer{},
	}
	n := 0
	e.sess = NewSession(SessionDeps{
		Settings: st, Secrets: e.secrets, Repo: e.repo, Clock: e.clock, Bus: b,
		Auth:     func(string) AuthAPI { return e.auth },
		Profiles: e.profiles, Tokens: e.tokens, Limiter: e.lim, Feed: e.feed, Probe: e.probe,
		Log:   slog.New(slog.NewJSONHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		NewID: func() string { n++; return fmt.Sprintf("sess-%d", n) },
	})
	return e
}

func (e *testEnv) configure() {
	e.t.Helper()
	if _, err := e.sess.SaveCredentials(context.Background(), testKey, testSecret); err != nil {
		e.t.Fatal(err)
	}
}

func (e *testEnv) seedToken(loginAt time.Time, acked bool) TokenRecord {
	e.t.Helper()
	ctx := context.Background()
	rec := TokenRecord{AccessToken: "acc-seeded-555", UserID: "AB1234", SessionID: "sess-seeded",
		LoggedInAtMs: loginAt.UnixMilli(), ExpiresAtMs: NextExpiry(loginAt).UnixMilli()}
	b, _ := json.Marshal(rec)
	if err := e.secrets.Set(secrets.KeyAccessToken, string(b)); err != nil {
		e.t.Fatal(err)
	}
	if err := e.repo.Insert(ctx, store.SessionRow{ID: rec.SessionID, UserID: rec.UserID, LoggedInAtMs: rec.LoggedInAtMs, ExpiresAtMs: rec.ExpiresAtMs, DataPlan: "connect"}); err != nil {
		e.t.Fatal(err)
	}
	if acked {
		if err := e.repo.AckRisk(ctx, rec.SessionID, rec.LoggedInAtMs+1000); err != nil {
			e.t.Fatal(err)
		}
	}
	return rec
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
