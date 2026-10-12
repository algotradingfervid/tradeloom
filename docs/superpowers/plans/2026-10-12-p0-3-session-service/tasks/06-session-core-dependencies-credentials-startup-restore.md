> **P0-3 Session & Service Layer Implementation Plan — Task 6.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 6: `Session` core — dependencies, credentials, startup restore, risk acknowledgement

**Files:**
- Create: `internal/kite/auth.go`, `internal/kite/session.go`, `internal/kite/client_key.go`, `internal/kite/session_relay.go`, `internal/kite/netguard.go`, `internal/kite/feed/feed.go`
- Test: `internal/kite/session_env_test.go` (shared fakes for Tasks 6–11), `internal/kite/session_test.go`, `internal/kite/client_key_test.go`, `internal/kite/session_relay_test.go`, `internal/kite/netguard_test.go`, `internal/kite/feed/feed_test.go`

**Interfaces:**
- Consumes: Tasks 1–5; P0-1 `settings.Get`, `settings.Set(ctx, …)`, `settings.Load`, `secrets.Store`/`ErrNotFound`, `bus.New(nil)`/`bus.Publish`, `logging.RegisterSecret`/`Go`; P0-2 `clock.Clock` (`At` returns a `clock.Timer`), `clock.NewFake`, `kite.Classify`/`KindSession`, `*kite.Err`, `kite.Client` (`New(Options)`, unexported fields `o`, `rec`, `api`, `hist`, `mu` — `client_key.go` is in the same package), `ratelimit.New(clk, ratelimit.Options{})`, `ticker.Options`, `ticker.New`, `(*ticker.Manager).Start/Stop/Reconnect/Subscribe/Unsubscribe/Release/Usage/Raw`, `ticker.KiteDialer`, `ticker.Conn`, `ticker.Events`, `ticker.Dialer`; `store.SessionsRepo`; gokiteconnect `kiteconnect.New`, `SetHTTPClient`, `SetBaseURI`, `SetAccessToken`, `GetLoginURL() string`, `GetLoginURLWithparams(url.Values) string` (verified `connect.go`, note the lower-case `p`), `GenerateSession(requestToken, apiSecret string) (UserSession, error)` (verified `user.go`: computes `checksum = sha256(api_key+request_token+api_secret)` itself), `InvalidateAccessToken() (bool, error)` (verified: `DELETE /session/token?api_key=…&access_token=…`).
- Produces:
  ```go
  type AuthSession struct{ UserID, AccessToken string }
  type AuthAPI interface {
      LoginURL(state string) string
      GenerateSession(ctx context.Context, requestToken, apiSecret string) (AuthSession, error)
      Invalidate(ctx context.Context, accessToken string) error
  }
  func NewAuth(hc *http.Client, baseURI string) func(apiKey string) AuthAPI
  type FeedControl interface{ Start(ctx context.Context, apiKey, accessToken string) error; Stop() }   // *feed.Feed
  type LimiterControl interface{ PauseAll(reason string); Resume() }                                  // *ratelimit.Limiter
  type TokenSink interface{ SetAPIKey(key string); SetAccessToken(token string) }                     // *kite.Client (SetAPIKey added here)
  type ProfileReader interface{ Profile(ctx context.Context) (kiteconnect.UserProfile, error) }       // *kite.Client
  type Prober interface{ Probe(ctx context.Context, apiKey, accessToken string) DataPlan }            // *CapabilityProbe (Task 9)
  type SessionDeps struct{ Settings *settings.Settings; Secrets secrets.Store; Repo *store.SessionsRepo; Clock clock.Clock; Bus *bus.Bus
      Auth func(apiKey string) AuthAPI; Profiles ProfileReader; Tokens TokenSink; Limiter LimiterControl; Feed FeedControl; Probe Prober
      Log *slog.Logger; ListenerTimeout time.Duration; NewID func() string }
  func NewSession(d SessionDeps) *Session
  func (s *Session) State() SessionState
  func (s *Session) CredentialsInfo() CredentialsInfo
  func (s *Session) SaveCredentials(ctx context.Context, apiKey, apiSecret string) (SessionState, error)
  func (s *Session) Restore(ctx context.Context) SessionState
  func (s *Session) AckRisk(ctx context.Context) (SessionState, error)
  // additions to P0-2's kite.Client and wiring glue (same package)
  func (c *Client) SetAPIKey(key string)                 // rebuilds c.api / c.hist exactly like New's mk; call before SetAccessToken
  type SessionErrorRelay struct{ mu sync.Mutex; fn func(error) }
  func (r *SessionErrorRelay) Set(fn func(error)); func (r *SessionErrorRelay) Handle(e *Err) // kite.New(Options{OnSessionError: relay.Handle}); later relay.Set(sess.HandleTokenError)
  var AllowedHosts = []string{"api.kite.trade", "kite.zerodha.com", "ws.kite.trade"}
  var ErrHostNotAllowed error
  func NewAllowListTransport(next http.RoundTripper, allowed []string) http.RoundTripper // refuses every other host (NFR-04); tests pass the mock host
  // package feed (internal/kite/feed): one ticker.Manager per login, subscriptions remembered across logins
  var ErrNotRunning error
  type Options struct{ Ticker ticker.Options /* Dial is ignored */; Root *url.URL; Log *slog.Logger }
  func New(o Options) *Feed
  func (f *Feed) Start(ctx context.Context, apiKey, accessToken string) error // builds ticker.New(Options{Dial: KiteDialer(apiKey, accessToken, Root)}), replays remembered subscriptions
  func (f *Feed) Stop(); func (f *Feed) Reconnect()
  func (f *Feed) Subscribe(c ticker.Consumer, owner string, tokens []uint32, mode ticker.Mode) error // remembered; forwarded when running; ErrNotRunning when stopped
  func (f *Feed) Unsubscribe(c ticker.Consumer, owner string, tokens []uint32); func (f *Feed) Release(c ticker.Consumer, owner string)
  func (f *Feed) Usage() ticker.Usage
  func (f *Feed) Ticks() <-chan ticker.Tick // raw ticks of every Manager this Feed starts (candle builder input)
  ```

gokiteconnect v4.4.3 takes no `context.Context`. The `*http.Client` timeout (P0-2's client: 10 s for GETs) bounds every call, and `ctx` is accepted so the signature survives a future library upgrade.

`feed` is its own package because `ticker` imports `kite` (for `TopicOrderUpdate`), so a per-login wrapper cannot live in `kite`. `*feed.Feed` satisfies `kite.FeedControl` (above), `service.LiveSubscriber` and `orders.LiveTokens` (Tasks 14, 19) structurally.

- [ ] **Step 1: Write the shared test fakes**

```go
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
func (f *fakeFeed) Stop()            { f.mu.Lock(); defer f.mu.Unlock(); f.stops++ }
func (f *fakeFeed) startCount() int { f.mu.Lock(); defer f.mu.Unlock(); return f.starts }
func (f *fakeFeed) stopCount() int  { f.mu.Lock(); defer f.mu.Unlock(); return f.stops }

type fakeLimiter struct {
	mu               sync.Mutex
	paused           bool
	pauses, resumes  int
}

func (f *fakeLimiter) PauseAll(string)  { f.mu.Lock(); defer f.mu.Unlock(); f.paused = true; f.pauses++ }
func (f *fakeLimiter) Resume()          { f.mu.Lock(); defer f.mu.Unlock(); f.paused = false; f.resumes++ }
func (f *fakeLimiter) isPaused() bool   { f.mu.Lock(); defer f.mu.Unlock(); return f.paused }
func (f *fakeLimiter) counts() (int, int) { f.mu.Lock(); defer f.mu.Unlock(); return f.pauses, f.resumes }

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

func (s *syncBuffer) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuffer) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

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
		Log:      slog.New(slog.NewJSONHandler(e.logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		NewID:    func() string { n++; return fmt.Sprintf("sess-%d", n) },
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
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/kite/session_test.go
package kite

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
)

// FR-ACCT-005 AC1 (backend): a fresh install starts in no_credentials, and nothing may call Kite.
func TestRestore_FreshInstall_NoCredentials(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 0))
	st := e.sess.Restore(context.Background())
	if st.State != StateNoCredentials {
		t.Fatalf("state %s", st.State)
	}
	if !e.lim.isPaused() {
		t.Fatal("limiter must be paused without a session")
	}
}

// FR-ACCT-005 AC2: the secret lives only in the secret store.
func TestSaveCredentials_SecretOnlyInSecretStore(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 0))
	st, err := e.sess.SaveCredentials(context.Background(), "  "+testKey+"\n", "\""+testSecret+"\"")
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateLoggedOut {
		t.Fatalf("state %s", st.State)
	}
	if v, _ := e.secrets.Get(secrets.KeyAPISecret); v != testSecret {
		t.Fatalf("secret store has %q", v)
	}
	if k := settings.Get(e.settings, SettingAPIKey); k != testKey {
		t.Fatalf("api key setting %q", k)
	}
	info := e.sess.CredentialsInfo()
	if !info.Configured || info.APIKeyMasked != "••••abcd" || info.RedirectURL != "http://127.0.0.1:7420/kite/callback" {
		t.Fatalf("info %+v", info)
	}
	err = filepath.WalkDir(filepath.Dir(e.db.Path()), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if bytes.Contains(b, []byte(testSecret)) {
			t.Errorf("secret found in %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(e.logs.String(), testSecret) {
		t.Fatal("secret found in logs")
	}
}

func TestSaveCredentials_RejectsInvalidAndStoresNothing(t *testing.T) {
	cases := map[string][2]string{
		"short key":         {"ab", testSecret},
		"secret with space": {testKey, "abc def ghi"},
		"identical":         {testKey, testKey},
	}
	for name, kv := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, ist(2026, 10, 12, 9, 0))
			_, err := e.sess.SaveCredentials(context.Background(), kv[0], kv[1])
			var le *LoginError
			if !errors.As(err, &le) || le.Code != CodeInvalidCredentials {
				t.Fatalf("err %v", err)
			}
			if e.secrets.has(secrets.KeyAPISecret) || settings.Get(e.settings, SettingAPIKey) != "" {
				t.Fatal("stored despite validation error")
			}
		})
	}
}

// FR-ACCT-006 AC4 and FR-SHELL-020 AC3 (data): a restart at 14:00 after a 09:00 login needs no login or ack.
func TestRestore_ValidTokenSkipsLogin(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 14, 0))
	e.configure()
	rec := e.seedToken(ist(2026, 10, 12, 9, 0), true)
	st := e.sess.Restore(context.Background())
	if st.State != StateReady || st.SessionID != rec.SessionID || st.RiskAckRequired || st.Offline {
		t.Fatalf("state %+v", st)
	}
	if st.ExpiresAtMs != ist(2026, 10, 13, 6, 0).UnixMilli() {
		t.Fatalf("expires %d", st.ExpiresAtMs)
	}
	if e.auth.exchangeCount() != 0 {
		t.Fatal("restore must not exchange a request token")
	}
	if e.tokens.get() != rec.AccessToken || e.lim.isPaused() {
		t.Fatal("token not applied or limiter still paused")
	}
	eventually(t, "feed start", func() bool { return e.feed.startCount() == 1 })
	eventually(t, "plan connect", func() bool { return e.sess.State().DataPlan == PlanConnect })
}

func TestRestore_AfterSixAM_IsExpired(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 13, 6, 30))
	e.configure()
	rec := e.seedToken(ist(2026, 10, 12, 9, 0), true)
	st := e.sess.Restore(context.Background())
	if st.State != StateExpired || st.ErrorCode != CodeSessionExpired || st.ErrorMessage != "Kite session expired — log in to continue." {
		t.Fatalf("state %+v", st)
	}
	if e.secrets.has(secrets.KeyAccessToken) || !e.lim.isPaused() || e.profiles.callCount() != 0 {
		t.Fatal("expired token must be deleted, limiter paused, and Kite never called")
	}
	row, err := e.repo.Get(context.Background(), rec.SessionID)
	if err != nil || row.EndReason != CodeSessionExpired {
		t.Fatalf("row %+v err %v", row, err)
	}
}

func TestRestore_TokenRevokedElsewhere_IsExpired(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 14, 0))
	e.configure()
	e.seedToken(ist(2026, 10, 12, 9, 0), true)
	e.profiles.setErr(APIError(kiteconnect.TokenError, 403, "Incorrect `api_key` or `access_token`."))
	st := e.sess.Restore(context.Background())
	if st.State != StateExpired || st.ErrorCode != CodeTokenRejected {
		t.Fatalf("state %+v", st)
	}
	if e.feed.startCount() != 0 || e.secrets.has(secrets.KeyAccessToken) {
		t.Fatal("feed started or token kept after rejection")
	}
}

func TestRestore_KiteUnreachable_StaysReadyOffline(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 14, 0))
	e.configure()
	e.seedToken(ist(2026, 10, 12, 9, 0), true)
	e.profiles.setErr(APIError(kiteconnect.NetworkError, 503, "Request failed."))
	st := e.sess.Restore(context.Background())
	if st.State != StateReady || !st.Offline {
		t.Fatalf("state %+v", st)
	}
}

// FR-ACCT-007 (data): a Personal-plan key never starts the ticker.
func TestRestore_PersonalPlan_DoesNotStartFeed(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 14, 0))
	e.configure()
	e.seedToken(ist(2026, 10, 12, 9, 0), true)
	e.probe.set(PlanNone)
	e.sess.Restore(context.Background())
	eventually(t, "plan none", func() bool { return e.sess.State().DataPlan == PlanNone })
	if e.feed.startCount() != 0 {
		t.Fatal("ticker started without a data plan")
	}
}

// FR-SHELL-020 (data): the acknowledgement is recorded against the session id.
func TestAckRisk(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 14, 0))
	e.configure()
	rec := e.seedToken(ist(2026, 10, 12, 9, 0), false)
	if st := e.sess.Restore(context.Background()); !st.RiskAckRequired {
		t.Fatalf("ack should be required: %+v", st)
	}
	st, err := e.sess.AckRisk(context.Background())
	if err != nil || st.RiskAckRequired {
		t.Fatalf("st %+v err %v", st, err)
	}
	row, _ := e.repo.Get(context.Background(), rec.SessionID)
	if row.RiskAckAtMs != ist(2026, 10, 12, 14, 0).UnixMilli() {
		t.Fatalf("ack at %d", row.RiskAckAtMs)
	}
}
```

```go
// internal/kite/client_key_test.go
package kite

import (
	"strings"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ratelimit"
)

// SetAPIKey rebuilds both inner gokiteconnect clients (10 s api, 30 s hist) on the same recording transport.
func TestSetAPIKey_RebuildsInnerClients(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 10, 12, 9, 0, 0, 0, clock.IST))
	c := New(Options{APIKey: "key1", BaseURI: "http://127.0.0.1:1", Limiter: ratelimit.New(clk, ratelimit.Options{}), Clock: clk})
	rec := c.rec
	c.SetAPIKey("key2")
	if c.APIKey() != "key2" {
		t.Fatalf("APIKey() = %q", c.APIKey())
	}
	for name, kc := range map[string]*kiteconnect.Client{"api": c.api, "hist": c.hist} {
		if u := kc.GetLoginURL(); !strings.Contains(u, "api_key=key2") {
			t.Fatalf("%s client still uses the old key: %s", name, u)
		}
	}
	if c.rec != rec {
		t.Fatal("the recording transport must survive a key change")
	}
}
```

```go
// internal/kite/session_relay_test.go
package kite

import (
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

func TestSessionErrorRelay_ForwardsOnceSet(t *testing.T) {
	var r SessionErrorRelay
	r.Handle(&Err{Kind: KindSession, Op: "Orders"}) // no target yet: must not panic
	var got []error
	r.Set(func(err error) { got = append(got, err) })
	e := &Err{Kind: KindSession, Op: "Orders", Code: 403, Type: kiteconnect.TokenError, Message: "Incorrect `api_key` or `access_token`."}
	r.Handle(e)
	r.Handle(nil)
	if len(got) != 1 || got[0] != e {
		t.Fatalf("got %v", got)
	}
	if !isTokenError(got[0]) {
		t.Fatal("a *kite.Err of KindSession must count as a token error")
	}
}
```

```go
// internal/kite/netguard_test.go
package kite

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAllowListTransport_RefusesOtherHosts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	hc := &http.Client{Transport: NewAllowListTransport(http.DefaultTransport, []string{u.Hostname()})}
	resp, err := hc.Get(srv.URL + "/user/profile")
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("allowed host: %v %v", resp, err)
	}
	resp.Body.Close()
	_, err = hc.Get("http://example.invalid/orders")
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("other host: %v", err)
	}
	if len(AllowedHosts) != 3 || AllowedHosts[0] != "api.kite.trade" || AllowedHosts[1] != "kite.zerodha.com" || AllowedHosts[2] != "ws.kite.trade" {
		t.Fatalf("AllowedHosts = %v", AllowedHosts)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestRestore|TestSaveCredentials|TestAckRisk|TestSetAPIKey|TestSessionErrorRelay|TestAllowListTransport' -v`
Expected: FAIL to compile with `undefined: NewSession`, `undefined: SessionDeps`, `undefined: AuthSession`, `c.SetAPIKey undefined`, `undefined: SessionErrorRelay`, `undefined: NewAllowListTransport`.

- [ ] **Step 4: Write the implementation**

```go
// internal/kite/auth.go
package kite

import (
	"context"
	"net/http"
	"net/url"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// AuthSession is the part of kiteconnect.UserSession the app keeps.
type AuthSession struct {
	UserID      string
	AccessToken string
}

// AuthAPI covers the three session calls. None of them is an order write; they are allowed in internal/kite (ARCH §5.2).
type AuthAPI interface {
	LoginURL(state string) string
	GenerateSession(ctx context.Context, requestToken, apiSecret string) (AuthSession, error)
	Invalidate(ctx context.Context, accessToken string) error
}

// NewAuth returns a factory of AuthAPI bound to an API key. baseURI "" keeps https://api.kite.trade (tests pass a fake).
func NewAuth(hc *http.Client, baseURI string) func(apiKey string) AuthAPI {
	return func(apiKey string) AuthAPI {
		c := kiteconnect.New(apiKey)
		if hc != nil {
			c.SetHTTPClient(hc)
		}
		if baseURI != "" {
			c.SetBaseURI(baseURI)
		}
		return &kcAuth{c: c}
	}
}

type kcAuth struct{ c *kiteconnect.Client }

func (a *kcAuth) LoginURL(state string) string {
	return a.c.GetLoginURLWithparams(url.Values{"state": {state}})
}

func (a *kcAuth) GenerateSession(_ context.Context, requestToken, apiSecret string) (AuthSession, error) {
	us, err := a.c.GenerateSession(requestToken, apiSecret)
	if err != nil {
		return AuthSession{}, err
	}
	return AuthSession{UserID: us.UserID, AccessToken: us.AccessToken}, nil
}

func (a *kcAuth) Invalidate(_ context.Context, accessToken string) error {
	a.c.SetAccessToken(accessToken)
	_, err := a.c.InvalidateAccessToken()
	return err
}
```

```go
// internal/kite/session.go
package kite

import (
	"context"
	"encoding/json"
	"errors"
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
	nonces map[string]bool
	done   map[string]bool // request tokens already exchanged in this run (reload-safe)
	gen    int             // login attempt generation
	runCtx context.Context

	exMu sync.Mutex // serialises token exchange and logout
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
	return &Session{
		d:      d,
		st:     SessionState{State: StateNoCredentials, DataPlan: PlanUnknown},
		prior:  StateLoggedOut,
		nonces: map[string]bool{},
		done:   map[string]bool{},
		runCtx: context.Background(),
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

func (s *Session) secret() (string, bool) {
	v, err := s.d.Secrets.Get(secrets.KeyAPISecret)
	return v, err == nil && v != ""
}

func (s *Session) configured() bool {
	_, ok := s.secret()
	return ok && s.apiKey() != ""
}

// idle is the resting state when no valid token exists.
func (s *Session) idle(code, msg string) SessionState {
	st := SessionState{State: StateLoggedOut, DataPlan: PlanUnknown, ErrorCode: code, ErrorMessage: msg}
	if !s.configured() {
		st.State = StateNoCredentials
	}
	return st
}

func readyState(rec TokenRecord, plan DataPlan, ackRequired, offline bool) SessionState {
	return SessionState{State: StateReady, SessionID: rec.SessionID, UserID: rec.UserID, LoggedInAtMs: rec.LoggedInAtMs,
		ExpiresAtMs: rec.ExpiresAtMs, DataPlan: plan, RiskAckRequired: ackRequired, Offline: offline}
}

func (s *Session) CredentialsInfo() CredentialsInfo {
	port := settings.Get(s.d.Settings, SettingRedirectPort)
	return CredentialsInfo{Configured: s.configured(), APIKeyMasked: MaskKey(s.apiKey()), Port: port,
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
		return s.State(), &LoginError{Code: CodeSecretStore, Message: "The API secret could not be saved to the secret store: " + err.Error()}
	}
	if err := settings.Set(ctx, s.d.Settings, SettingAPIKey, k); err != nil {
		return s.State(), err
	}
	s.d.Tokens.SetAPIKey(k)
	s.d.Log.Info("kite credentials saved", "api_key", MaskKey(k), "secret_backend", s.d.Secrets.Backend())
	return s.set(s.idle("", "")), nil
}

// Restore resumes a still-valid session at startup without asking for a login (FR-ACCT-006 AC4).
func (s *Session) Restore(ctx context.Context) SessionState {
	if !s.configured() {
		s.d.Limiter.PauseAll("no credentials")
		return s.set(s.idle("", ""))
	}
	apiKey := s.apiKey()
	s.d.Tokens.SetAPIKey(apiKey)
	raw, err := s.d.Secrets.Get(secrets.KeyAccessToken)
	if err != nil {
		if !errors.Is(err, secrets.ErrNotFound) {
			s.d.Log.Warn("reading the access token failed", "err", err)
		}
		s.d.Limiter.PauseAll("logged out")
		return s.set(s.idle("", ""))
	}
	var rec TokenRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil || rec.AccessToken == "" || rec.SessionID == "" {
		_ = s.d.Secrets.Delete(secrets.KeyAccessToken)
		s.d.Limiter.PauseAll("logged out")
		return s.set(s.idle("", ""))
	}
	logging.RegisterSecret(rec.AccessToken)
	if s.d.Clock.Now().UnixMilli() >= rec.ExpiresAtMs {
		return s.endSession(rec, CodeSessionExpired)
	}
	s.d.Tokens.SetAccessToken(rec.AccessToken)
	s.d.Limiter.Resume()
	offline := false
	if _, err := s.d.Profiles.Profile(ctx); err != nil {
		if isTokenError(err) {
			return s.endSession(rec, CodeTokenRejected)
		}
		offline = true
		s.d.Log.Warn("Kite unreachable at startup; continuing with cached data", "kind", errKind(err))
	}
	plan, ackRequired := PlanUnknown, true
	if row, err := s.d.Repo.Get(ctx, rec.SessionID); err == nil {
		if row.DataPlan != "" {
			plan = DataPlan(row.DataPlan)
		}
		ackRequired = row.RiskAckAtMs == 0
	}
	s.mu.Lock()
	s.rec = &rec
	s.mu.Unlock()
	st := s.set(readyState(rec, plan, ackRequired, offline))
	s.afterReady(apiKey, rec)
	return st
}

// AckRisk records the FR-SHELL-020 acknowledgement for the current session id.
func (s *Session) AckRisk(ctx context.Context) (SessionState, error) {
	st := s.State()
	if st.State != StateReady {
		return st, &LoginError{Code: CodeNotLoggedIn, Message: "Log in to Kite first."}
	}
	if err := s.d.Repo.AckRisk(ctx, st.SessionID, s.d.Clock.Now().UnixMilli()); err != nil {
		return st, err
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
		if !s.applyPlan(rec.SessionID, plan) {
			return
		}
		if plan == PlanNone {
			s.d.Log.Info("API key has no market-data permission; running in orders-and-portfolio-only mode")
			return
		}
		if err := s.d.Feed.Start(ctx, apiKey, rec.AccessToken); err != nil {
			s.d.Log.Warn("ticker start failed", "err", err)
		}
	})
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
	if err := s.d.Repo.SetDataPlan(context.Background(), sessionID, string(plan)); err != nil {
		s.d.Log.Warn("recording the data plan failed", "err", err)
	}
	bus.Publish(s.d.Bus, TopicSessionState, st)
	return true
}

// endSession performs the side effects of a session ending through expiry or rejection.
// Callers guarantee exclusivity (the state is no longer ready).
func (s *Session) endSession(rec TokenRecord, code string) SessionState {
	s.d.Limiter.PauseAll("session expired")
	s.d.Feed.Stop()
	s.d.Tokens.SetAccessToken("")
	if err := s.d.Secrets.Delete(secrets.KeyAccessToken); err != nil && !errors.Is(err, secrets.ErrNotFound) {
		s.d.Log.Warn("deleting the access token failed", "err", err)
	}
	s.endRow(rec.SessionID, code)
	s.mu.Lock()
	s.rec = nil
	s.mu.Unlock()
	s.d.Log.Info("kite session ended", "reason", code)
	return s.set(SessionState{State: StateExpired, UserID: rec.UserID, DataPlan: PlanUnknown, ErrorCode: code, ErrorMessage: msgExpired})
}

func (s *Session) endRow(sessionID, reason string) {
	if err := s.d.Repo.End(context.Background(), sessionID, s.d.Clock.Now().UnixMilli(), reason); err != nil {
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
```

```go
// internal/kite/client_key.go
package kite

import (
	"net/http"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// SetAPIKey installs a new API key (Settings → Kite connection) by rebuilding both inner gokiteconnect
// clients exactly as New's mk does, on the same recording transport. Like New it leaves the clients
// without an access token, so the Session always calls it before SetAccessToken.
func (c *Client) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.o.APIKey = key
	mk := func(timeout time.Duration) *kiteconnect.Client {
		kc := kiteconnect.New(key)
		kc.SetHTTPClient(&http.Client{Timeout: timeout, Transport: c.rec})
		if c.o.BaseURI != "" {
			kc.SetBaseURI(c.o.BaseURI)
		}
		return kc
	}
	c.api, c.hist = mk(10*time.Second), mk(30*time.Second)
}
```

```go
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

// Handle forwards a session error to the target; before Set it drops the error.
func (r *SessionErrorRelay) Handle(e *Err) {
	r.mu.Lock()
	fn := r.fn
	r.mu.Unlock()
	if fn != nil && e != nil {
		fn(e)
	}
}
```

```go
// internal/kite/netguard.go
package kite

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// AllowedHosts is the NFR-04 network allow-list: the REST API, the login page and the WebSocket feed.
var AllowedHosts = []string{"api.kite.trade", "kite.zerodha.com", "ws.kite.trade"}

// ErrHostNotAllowed is returned (wrapped) for any request to a host outside the allow-list.
var ErrHostNotAllowed = errors.New("kite: host is not on the network allow-list")

type allowListTransport struct {
	next    http.RoundTripper
	allowed map[string]bool
}

// NewAllowListTransport refuses every request whose host is not in allowed before it reaches next.
// cmd/tradeloom passes AllowedHosts; tests pass the mock server's host.
func NewAllowListTransport(next http.RoundTripper, allowed []string) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	m := make(map[string]bool, len(allowed))
	for _, h := range allowed {
		m[strings.ToLower(h)] = true
	}
	return &allowListTransport{next: next, allowed: m}
}

func (t *allowListTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := strings.ToLower(req.URL.Hostname())
	if !t.allowed[host] {
		return nil, fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
	}
	return t.next.RoundTrip(req)
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/kite/ -run 'TestRestore|TestSaveCredentials|TestAckRisk|TestSetAPIKey|TestSessionErrorRelay|TestAllowListTransport' -v`
Expected: PASS (all 12 tests; `-race` clean).

- [ ] **Step 6: Write the failing feed test**

```go
// internal/kite/feed/feed_test.go
package feed

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ticker"
)

// fakeConn is a ticker.Conn that "connects" at once and records what the Manager asks of it.
type fakeConn struct {
	mu     sync.Mutex
	ev     ticker.Events
	tokens map[uint32]bool
	stops  int
}

func (c *fakeConn) Serve(ctx context.Context) {
	if c.ev.OnConnect != nil {
		c.ev.OnConnect()
	}
	<-ctx.Done()
}
func (c *fakeConn) Subscribe(tokens []uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range tokens {
		c.tokens[t] = true
	}
	return nil
}
func (c *fakeConn) Unsubscribe(tokens []uint32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range tokens {
		delete(c.tokens, t)
	}
	return nil
}
func (c *fakeConn) SetMode(ticker.Mode, []uint32) error { return nil }
func (c *fakeConn) Stop()                                { c.mu.Lock(); c.stops++; c.mu.Unlock() }
func (c *fakeConn) sorted() []uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]uint32, 0, len(c.tokens))
	for t := range c.tokens {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

type fakeDials struct {
	mu    sync.Mutex
	conns []*fakeConn
	creds []string
}

func (d *fakeDials) dialFor(apiKey, accessToken string) ticker.Dialer {
	d.mu.Lock()
	d.creds = append(d.creds, apiKey+":"+accessToken)
	d.mu.Unlock()
	return func(ev ticker.Events) ticker.Conn {
		c := &fakeConn{ev: ev, tokens: map[uint32]bool{}}
		d.mu.Lock()
		d.conns = append(d.conns, c)
		d.mu.Unlock()
		return c
	}
}

func (d *fakeDials) last() *fakeConn {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.conns) == 0 {
		return nil
	}
	return d.conns[len(d.conns)-1]
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

func newFeed(t *testing.T) (*Feed, *fakeDials) {
	t.Helper()
	d := &fakeDials{}
	f := New(Options{Ticker: ticker.Options{Clock: clock.NewFake(time.Date(2026, 10, 12, 9, 15, 0, 0, clock.IST)), PerConn: 4, MaxConns: 1}})
	f.dialFor = d.dialFor
	return f, d
}

func equal(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Subscriptions made before login are remembered and applied when the feed starts; Stop makes the feed
// refuse again without forgetting them (the next login replays them).
func TestFeed_RemembersAndReplaysAcrossLogins(t *testing.T) {
	f, d := newFeed(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := f.Subscribe(ticker.ConsumerWatchlist, "wl-1", []uint32{1, 2}, ticker.ModeQuote); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("before start: %v", err)
	}
	if u := f.Usage(); u.Used != 0 || u.Limit != 4 {
		t.Fatalf("usage before start %+v", u)
	}
	if err := f.Start(ctx, "key", "tok-1"); err != nil {
		t.Fatal(err)
	}
	if d.creds[0] != "key:tok-1" {
		t.Fatalf("dialer credentials %v", d.creds)
	}
	eventually(t, "replayed subscriptions", func() bool { c := d.last(); return c != nil && equal(c.sorted(), []uint32{1, 2}) })
	if err := f.Subscribe(ticker.ConsumerChart, "chart-1", []uint32{3}, ticker.ModeQuote); err != nil {
		t.Fatal(err)
	}
	eventually(t, "live subscribe", func() bool { return equal(d.last().sorted(), []uint32{1, 2, 3}) })
	if u := f.Usage(); u.Used != 3 {
		t.Fatalf("usage %+v", u)
	}
	var budget *ticker.ErrBudget
	if err := f.Subscribe(ticker.ConsumerChart, "chart-2", []uint32{4, 5}, ticker.ModeQuote); !errors.As(err, &budget) {
		t.Fatalf("over budget: %v", err)
	}
	f.Unsubscribe(ticker.ConsumerWatchlist, "wl-1", []uint32{2})
	f.Release(ticker.ConsumerChart, "chart-1")
	eventually(t, "unsubscribed", func() bool { return equal(d.last().sorted(), []uint32{1}) })

	f.Stop()
	if err := f.Subscribe(ticker.ConsumerWidget, "portfolio", []uint32{7}, ticker.ModeQuote); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("after stop: %v", err)
	}
	if u := f.Usage(); u.Used != 0 {
		t.Fatalf("usage after stop %+v", u)
	}
	if err := f.Start(ctx, "key", "tok-2"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "second login replay", func() bool { return len(d.conns) == 2 && equal(d.last().sorted(), []uint32{1, 7}) })
	if d.conns[0].stops == 0 {
		t.Fatal("the first login's connection was not stopped")
	}
}

func TestFeed_TicksStreamSurvivesRestart(t *testing.T) {
	f, d := newFeed(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := f.Start(ctx, "key", "tok-1"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "connected", func() bool { return d.last() != nil })
	f.Stop()
	if err := f.Start(ctx, "key", "tok-2"); err != nil {
		t.Fatal(err)
	}
	if f.Ticks() == nil {
		t.Fatal("Ticks must be one long-lived channel")
	}
	select {
	case tk := <-f.Ticks():
		t.Fatalf("unexpected tick %+v", tk)
	case <-time.After(20 * time.Millisecond):
	}
}
```

- [ ] **Step 7: Write the feed**

```go
// internal/kite/feed/feed.go
// Package feed owns one ticker.Manager per Kite login. A Manager is single-use (P0-2: Stop is final and
// carries the access token in its dialer), so the Session starts a fresh one after every login while
// consumers keep subscribing to this long-lived Feed; their subscriptions are replayed on each Start.
package feed

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"sort"
	"sync"

	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/logging"
)

// ErrNotRunning is returned by Subscribe while no login is active. The subscription is still
// remembered and applied at the next Start; callers that need data now fall back to polling.
var ErrNotRunning = errors.New("feed: not running (no Kite session)")

const rawBuffer = 4096

// Options configures the Feed. Ticker.Dial is ignored: every Start builds ticker.KiteDialer with that login's credentials.
type Options struct {
	Ticker ticker.Options
	Root   *url.URL // nil = Kite's production WebSocket root; tests pass mock.Server.TickerURL()
	Log    *slog.Logger
}

type ownerKey struct {
	c     ticker.Consumer
	owner string
}

// Feed is safe for concurrent use.
type Feed struct {
	o       Options
	dialFor func(apiKey, accessToken string) ticker.Dialer
	ticks   chan ticker.Tick

	mu       sync.Mutex
	m        *ticker.Manager
	stopPump context.CancelFunc
	want     map[ownerKey]map[uint32]ticker.Mode
}

// New builds a stopped Feed.
func New(o Options) *Feed {
	if o.Log == nil {
		o.Log = o.Ticker.Log
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	f := &Feed{o: o, ticks: make(chan ticker.Tick, rawBuffer), want: map[ownerKey]map[uint32]ticker.Mode{}}
	f.dialFor = func(apiKey, accessToken string) ticker.Dialer { return ticker.KiteDialer(apiKey, accessToken, o.Root) }
	return f
}

// Start builds and starts a Manager for this login, pumps its raw ticks into Ticks() and replays every
// remembered subscription. A previous Manager is stopped first.
func (f *Feed) Start(ctx context.Context, apiKey, accessToken string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopLocked()
	opts := f.o.Ticker
	opts.Dial = f.dialFor(apiKey, accessToken)
	if opts.Log == nil {
		opts.Log = f.o.Log
	}
	m := ticker.New(opts)
	if err := m.Start(ctx); err != nil {
		m.Stop()
		return err
	}
	f.m = m
	raw, cancelRaw := m.Raw(rawBuffer)
	pumpCtx, cancelPump := context.WithCancel(ctx)
	f.stopPump = func() { cancelRaw(); cancelPump() }
	logging.Go("feed.pump", func() {
		for {
			select {
			case <-pumpCtx.Done():
				return
			case t := <-raw:
				select {
				case f.ticks <- t:
				default:
				}
			}
		}
	})
	for k, set := range f.want {
		for _, mode := range []ticker.Mode{ticker.ModeLTP, ticker.ModeQuote, ticker.ModeFull} {
			toks := tokensAt(set, mode)
			if len(toks) == 0 {
				continue
			}
			if err := m.Subscribe(k.c, k.owner, toks, mode); err != nil {
				f.o.Log.Warn("feed.replay_refused", "component", "feed", "consumer", k.c.String(), "owner", k.owner, "tokens", len(toks), "err", err)
			}
		}
	}
	return nil
}

func tokensAt(set map[uint32]ticker.Mode, mode ticker.Mode) []uint32 {
	var out []uint32
	for t, m := range set {
		if m == mode {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Stop ends the current login's Manager (logout, expiry, shutdown). Subscriptions stay remembered.
func (f *Feed) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopLocked()
}

func (f *Feed) stopLocked() {
	if f.stopPump != nil {
		f.stopPump()
		f.stopPump = nil
	}
	if f.m != nil {
		f.m.Stop()
		f.m = nil
	}
}

// Reconnect is the user's "Retry now" after the Manager went offline.
func (f *Feed) Reconnect() {
	f.mu.Lock()
	m := f.m
	f.mu.Unlock()
	if m != nil {
		m.Reconnect()
	}
}

// Subscribe remembers tokens for (c, owner) at mode and forwards them to the running Manager. A budget
// refusal (*ticker.ErrBudget) changes nothing; when stopped it returns ErrNotRunning after remembering.
func (f *Feed) Subscribe(c ticker.Consumer, owner string, tokens []uint32, mode ticker.Mode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m != nil {
		if err := f.m.Subscribe(c, owner, tokens, mode); err != nil {
			return err
		}
	}
	k := ownerKey{c, owner}
	set := f.want[k]
	if set == nil {
		set = map[uint32]ticker.Mode{}
		f.want[k] = set
	}
	for _, t := range tokens {
		if set[t] < mode {
			set[t] = mode
		}
	}
	if f.m == nil {
		return ErrNotRunning
	}
	return nil
}

// Unsubscribe forgets tokens of (c, owner) and forwards when running.
func (f *Feed) Unsubscribe(c ticker.Consumer, owner string, tokens []uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if set := f.want[ownerKey{c, owner}]; set != nil {
		for _, t := range tokens {
			delete(set, t)
		}
	}
	if f.m != nil {
		f.m.Unsubscribe(c, owner, tokens)
	}
}

// Release forgets every token of (c, owner) and forwards when running.
func (f *Feed) Release(c ticker.Consumer, owner string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.want, ownerKey{c, owner})
	if f.m != nil {
		f.m.Release(c, owner)
	}
}

// Usage is the running Manager's budget, or an empty budget of the configured size when stopped.
func (f *Feed) Usage() ticker.Usage {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m != nil {
		return f.m.Usage()
	}
	perConn, maxConns := f.o.Ticker.PerConn, f.o.Ticker.MaxConns
	if perConn == 0 {
		perConn = 3000
	}
	if maxConns == 0 {
		maxConns = 3
	}
	return ticker.Usage{Limit: perConn * maxConns, ByConsumer: map[string]int{}}
}

// Ticks is the raw tick stream across logins (the candle builder's Run input). A full buffer drops ticks.
func (f *Feed) Ticks() <-chan ticker.Tick { return f.ticks }
```

- [ ] **Step 8: Run the feed tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/kite/feed/ -v`
Expected: PASS (`TestFeed_RemembersAndReplaysAcrossLogins`, `TestFeed_TicksStreamSurvivesRestart`; `-race` clean).

- [ ] **Step 9: Commit**

```bash
git add internal/kite/auth.go internal/kite/session.go internal/kite/client_key.go internal/kite/session_relay.go internal/kite/netguard.go internal/kite/feed \
  internal/kite/session_env_test.go internal/kite/session_test.go internal/kite/client_key_test.go internal/kite/session_relay_test.go internal/kite/netguard_test.go
git commit -m "feat(kite): session core with credentials, restore and risk ack; per-login feed, key change, allow-list transport

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
