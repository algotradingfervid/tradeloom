package kite

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// Ruling 57 (Task 23): a redirect URL handed over by a second launch (argv) is not an explicit paste, so
// it follows the loopback callback rule: only the state nonce of the current login attempt is accepted.
func TestLogin_ForwardedRedirect_RequiresCurrentState(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	setPort(t, e, freePort(t))
	e.auth.allow("rtok12345678", AuthSession{UserID: "AB1234", AccessToken: "acc-live-777"})
	ls, err := e.sess.BeginLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	base := ls.RedirectURL + "?request_token=rtok12345678&action=login&status=success"
	for _, u := range []string{base, base + "&state=forgedstate", "rtok12345678"} {
		_, err := e.sess.CompleteForwardedRedirect(context.Background(), u)
		var le *LoginError
		if !errors.As(err, &le) || le.Code != CodeStateMismatch {
			t.Fatalf("%q: err %v", u, err)
		}
	}
	if e.auth.exchangeCount() != 0 || e.sess.State().State != StateLoggingIn {
		t.Fatalf("exchanged %d, state %s", e.auth.exchangeCount(), e.sess.State().State)
	}
	good := base + "&state=" + stateFrom(t, ls.LoginURL)
	st, err := e.sess.CompleteForwardedRedirect(context.Background(), good)
	if err != nil || st.State != StateReady {
		t.Fatalf("state %+v err %v", st, err)
	}
	// A repeated hand-off of the same URL (the user launched the link twice) does not exchange again.
	if _, err := e.sess.CompleteForwardedRedirect(context.Background(), good); err != nil || e.auth.exchangeCount() != 1 {
		t.Fatalf("repeat: err %v, exchanged %d", err, e.auth.exchangeCount())
	}
	if strings.Contains(e.logs.String(), "rtok12345678") {
		t.Fatal("request token leaked into logs")
	}
}

type plainSink struct{}

func (plainSink) SetAPIKey(string)      {}
func (plainSink) SetAccessToken(string) {}

// r70 review → Task 23: a TokenSink without TokenGen disables the stale-token guard; NewSession says so.
func TestNewSession_WarnsWhenTokensLackTokenGen(t *testing.T) {
	var buf bytes.Buffer
	NewSession(SessionDeps{Tokens: plainSink{}, Log: slog.New(slog.NewJSONHandler(&buf, nil))})
	if !strings.Contains(buf.String(), "TokenGen") {
		t.Fatalf("no warning: %s", buf.String())
	}
	buf.Reset()
	NewSession(SessionDeps{Tokens: &fakeTokens{}, Log: slog.New(slog.NewJSONHandler(&buf, nil))})
	if buf.Len() != 0 {
		t.Fatalf("unexpected log: %s", buf.String())
	}
}
