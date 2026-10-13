// internal/kite/ruling70_test.go
package kite

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/secrets"
)

const msgTokenRejected = "Incorrect `api_key` or `access_token`."

// tokenErrGen is a TokenException as Client.call reports it for an attempt that sent token generation gen.
func tokenErrGen(gen uint64) *Err {
	return &Err{Kind: KindSession, Op: "Orders", Code: 403, Type: kiteconnect.TokenError, Message: msgTokenRejected, TokenGen: gen}
}

// pasteLogin runs BeginLogin + CompleteLogin with request token rt that Kite exchanges for access token tok.
func pasteLogin(t *testing.T, e *testEnv, rt, tok string) SessionState {
	t.Helper()
	e.auth.allow(rt, AuthSession{UserID: "AB1234", AccessToken: tok})
	if _, err := e.sess.BeginLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, err := e.sess.CompleteLogin(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// day1Then2 logs in on day 1, lets the 06:00 expiry end that session and logs in again on day 2. It
// returns the day-1 token generation and the day-2 state.
func day1Then2(t *testing.T, e *testEnv) (uint64, SessionState) {
	t.Helper()
	e.configure()
	setPort(t, e, freePort(t))
	pasteLogin(t, e, "rtokday1aaaa", "acc-day1-111")
	day1 := e.tokens.TokenGen()
	e.clock.Set(ist(2026, 10, 13, 8, 0))
	e.sess.checkExpiry()
	if st := e.sess.State(); st.State != StateExpired {
		t.Fatalf("day 1 did not expire: %+v", st)
	}
	st := pasteLogin(t, e, "rtokday2bbbb", "acc-day2-222")
	if st.State != StateReady || e.tokens.TokenGen() == day1 {
		t.Fatalf("day-2 login %+v (gen %d, day-1 gen %d)", st, e.tokens.TokenGen(), day1)
	}
	return day1, st
}

func assertReadyOn(t *testing.T, e *testEnv, st SessionState, tok string) {
	t.Helper()
	got := e.sess.State()
	if got.State != StateReady || got.SessionID != st.SessionID {
		t.Fatalf("day-2 session ended: %+v (want ready %s)", got, st.SessionID)
	}
	if e.tokens.get() != tok || !e.secrets.has(secrets.KeyAccessToken) || e.lim.isPaused() {
		t.Fatalf("day-2 token cleared (mem %q, stored %v) or limiter paused (%v)", e.tokens.get(), e.secrets.has(secrets.KeyAccessToken), e.lim.isPaused())
	}
}

// Ruling 70 (a): a day-1 call in flight across the day-2 login returns TokenException afterwards; the
// relay delivers it, and the day-2 session stays ready. The ignore is logged without any token.
func TestStaleTokenException_AfterDay2Login_KeepsDay2Ready(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	day1, st := day1Then2(t, e)
	var relay SessionErrorRelay
	relay.Set(e.sess.HandleTokenError)
	relay.Handle(tokenErrGen(day1))
	assertReadyOn(t, e, st, "acc-day2-222")
	logs := e.logs.String()
	if !strings.Contains(logs, "earlier access token") {
		t.Fatalf("stale error not logged: %s", logs)
	}
	for _, tok := range []string{"acc-day1-111", "acc-day2-222"} {
		if strings.Contains(logs, tok) {
			t.Fatalf("log line holds a token: %s", logs)
		}
	}
}

// Ruling 70 (b): a TokenException from the current token generation still ends the session, and so does
// one without a generation (TokenGen 0: built by tests or an older path), as before Ruling 70.
func TestTokenException_CurrentOrUnknownGen_EndsSession(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  func(e *testEnv) error
	}{
		{"current generation", func(e *testEnv) error { return tokenErrGen(e.tokens.TokenGen()) }},
		{"no generation (*Err)", func(*testEnv) error { return tokenErrGen(0) }},
		{"no generation (kiteconnect.Error)", func(*testEnv) error { return APIError(kiteconnect.TokenError, 403, msgTokenRejected) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, ist(2026, 10, 12, 9, 2))
			_, st := day1Then2(t, e)
			e.sess.HandleTokenError(tc.err(e))
			got := e.sess.State()
			if got.State != StateExpired || got.ErrorCode != CodeTokenRejected {
				t.Fatalf("state %+v, want expired/token_rejected", got)
			}
			if e.tokens.get() != "" || e.secrets.has(secrets.KeyAccessToken) || !e.lim.isPaused() {
				t.Fatal("token kept or limiter running after a current-generation rejection")
			}
			row, err := e.repo.Get(context.Background(), st.SessionID)
			if err != nil || row.EndReason != CodeTokenRejected {
				t.Fatalf("day-2 row %+v %v", row, err)
			}
		})
	}
}

// Ruling 70 (c): stale day-1 errors race the day-1 expiry and the day-2 login; whatever the interleaving,
// the day-2 session ends ready. Run under -race.
func TestStaleTokenException_RacesDay2Login(t *testing.T) {
	for i := 0; i < 25; i++ {
		e := newEnv(t, ist(2026, 10, 12, 9, 2))
		e.configure()
		setPort(t, e, freePort(t))
		pasteLogin(t, e, "rtokday1aaaa", "acc-day1-111")
		day1 := e.tokens.TokenGen()
		e.auth.allow("rtokday2bbbb", AuthSession{UserID: "AB1234", AccessToken: "acc-day2-222"})
		var relay SessionErrorRelay
		relay.Set(e.sess.HandleTokenError)

		var stop atomic.Bool
		var wg sync.WaitGroup
		for g := 0; g < 4; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for n := 0; n < 200 && !stop.Load(); n++ {
					relay.Handle(tokenErrGen(day1))
				}
			}()
		}
		e.clock.Set(ist(2026, 10, 13, 8, 0))
		e.sess.checkExpiry() // may find day 1 already ended by one of the day-1 errors
		var st SessionState
		for st.State != StateReady {
			if _, err := e.sess.BeginLogin(context.Background()); err != nil {
				t.Fatal(err)
			}
			var err error
			if st, err = e.sess.CompleteLogin(context.Background(), "rtokday2bbbb"); err != nil {
				t.Fatal(err)
			}
		}
		for n := 0; n < 50; n++ { // late arrivals after the commit
			relay.Handle(tokenErrGen(day1))
		}
		stop.Store(true)
		wg.Wait()
		assertReadyOn(t, e, st, "acc-day2-222")
	}
}

// Ruling 70 (a), end to end with the real Client: a day-1 Orders call is held by the server across the
// day-1 expiry and the day-2 login, then answered with TokenException. The Client tags the error with
// the day-1 generation, does not pause the queues of the day-2 token, and the Session ignores it.
func TestStaleTokenException_RealClient_InFlightAcrossDay2Login(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/orders":
			once.Do(func() { close(entered) })
			<-release
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "error_type": "TokenException", "message": msgTokenRejected})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"user_id": "AB1234"}})
		}
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clk := clock.New(clock.Options{})
	lim := ratelimit.New(clk, ratelimit.Options{})
	lim.Run(ctx)
	var relay SessionErrorRelay
	cli := New(Options{APIKey: testKey, BaseURI: srv.URL, Limiter: lim, Clock: clk, OnSessionError: relay.Handle,
		Jitter: func() time.Duration { return time.Millisecond }})

	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.sess.d.Tokens, e.sess.d.Profiles, e.sess.d.Limiter = cli, cli, lim
	relay.Set(e.sess.HandleTokenError)
	e.configure()
	setPort(t, e, freePort(t))
	pasteLogin(t, e, "rtokday1aaaa", "acc-day1-111")
	day1 := cli.TokenGen()

	res := make(chan error, 1)
	go func() { _, err := cli.Orders(context.Background()); res <- err }()
	<-entered
	e.clock.Set(ist(2026, 10, 13, 8, 0))
	e.sess.checkExpiry()
	st := pasteLogin(t, e, "rtokday2bbbb", "acc-day2-222")
	close(release)
	err := <-res

	var ke *Err
	if !errors.As(err, &ke) || ke.Kind != KindSession || ke.TokenGen != day1 || cli.TokenGen() == day1 {
		t.Fatalf("err %v (gen %d), day-1 gen %d, current gen %d", err, ke.TokenGen, day1, cli.TokenGen())
	}
	if got := e.sess.State(); got.State != StateReady || got.SessionID != st.SessionID {
		t.Fatalf("day-2 session ended by a day-1 error: %+v", got)
	}
	if lim.Snapshot().Paused {
		t.Fatal("a day-1 TokenException paused the day-2 queues")
	}
	pctx, pcancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pcancel()
	if _, err := cli.Profile(pctx); err != nil {
		t.Fatalf("day-2 call after the stale error: %v", err)
	}
}
