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
