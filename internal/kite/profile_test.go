// internal/kite/profile_test.go
package kite

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// FR-ACCT-001 AC1 (data).
func TestProfile_MatchesKiteAndIsReadOnlyCopy(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	e.profiles.p = kiteconnect.UserProfile{UserID: "AB1234", UserName: "Asha Bhat", UserShortName: "Asha", Email: "asha@example.com",
		Broker: "ZERODHA", Exchanges: []string{"NSE", "BSE", "NFO"}, Products: []string{"CNC", "MIS", "NRML"},
		AvatarURL: "https://s3.ap-south-1.amazonaws.com/zerodha-kite-blobs/avatars/x.png"}
	if _, err := e.sess.Profile(context.Background()); err == nil {
		t.Fatal("profile must require a session")
	}
	loginViaPaste(t, e)
	p, err := e.sess.Profile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != "AB1234" || p.Name != "Asha Bhat" || p.Email != "asha@example.com" || p.Broker != "Zerodha" || p.Initials != "AB" {
		t.Fatalf("profile %+v", p)
	}
	if p.AvatarURL != "" {
		t.Fatalf("avatar from a non-allow-listed host must be dropped: %s", p.AvatarURL)
	}
	if strings.Join(p.Exchanges, ",") != "NSE,BSE,NFO" || strings.Join(p.Products, ",") != "CNC,MIS,NRML" {
		t.Fatalf("exchanges/products %+v", p)
	}
}

func TestProfile_LogValueRedacts(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("profile loaded", "profile", ProfileFrom(kiteconnect.UserProfile{UserID: "AB1234", UserName: "Asha Bhat", Email: "asha@example.com"}))
	out := buf.String()
	if strings.Contains(out, "asha@example.com") || strings.Contains(out, "Asha Bhat") || strings.Contains(out, "AB1234") || !strings.Contains(out, "AB****") {
		t.Fatalf("log line %s", out)
	}
}

func TestProfile_TokenExceptionExpiresSession(t *testing.T) {
	e := newEnv(t, ist(2026, 10, 12, 9, 2))
	e.configure()
	loginViaPaste(t, e)
	e.profiles.setErr(APIError(kiteconnect.TokenError, 403, "Incorrect `api_key` or `access_token`."))
	_, err := e.sess.Profile(context.Background())
	var le *LoginError
	if !errors.As(err, &le) {
		t.Fatalf("err %v", err)
	}
	eventually(t, "expired", func() bool { return e.sess.State().State == StateExpired })
}
