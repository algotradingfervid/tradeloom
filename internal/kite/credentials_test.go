// internal/kite/credentials_test.go
package kite

import (
	"context"
	"errors"
	"testing"
	"tradeloom/internal/logging"
	"tradeloom/internal/settings"
)

func TestNormalizeCredential(t *testing.T) {
	for in, want := range map[string]string{
		"  abc123def  ":   "abc123def",
		"\"abc123def\"\n": "abc123def",
		"'abc123def'":     "abc123def",
		"\tabc123def\r\n": "abc123def",
	} {
		if got := NormalizeCredential(in); got != want {
			t.Fatalf("NormalizeCredential(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateCredentials(t *testing.T) {
	ok := []string{"abcd12", "kitekey1234abcd", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz01"}
	bad := []string{"", "abc", "has space1", "semi;colon1", "ünïcode123", "x123456789012345678901234567890123456789012345678901234567890123456"}
	for _, k := range ok {
		if err := ValidateAPIKey(k); err != nil {
			t.Fatalf("ValidateAPIKey(%q) = %v, want nil", k, err)
		}
		if err := ValidateAPISecret(k); err != nil {
			t.Fatalf("ValidateAPISecret(%q) = %v, want nil", k, err)
		}
	}
	for _, k := range bad {
		var le *LoginError
		if err := ValidateAPIKey(k); !errors.As(err, &le) || le.Code != CodeInvalidCredentials {
			t.Fatalf("ValidateAPIKey(%q) = %v, want invalid_credentials", k, err)
		}
		if err := ValidateAPISecret(k); !errors.As(err, &le) || le.Code != CodeInvalidCredentials {
			t.Fatalf("ValidateAPISecret(%q) = %v, want invalid_credentials", k, err)
		}
	}
}

func TestValidatePort(t *testing.T) {
	for _, p := range []int{1024, 7420, 65535} {
		if err := ValidatePort(p); err != nil {
			t.Fatalf("port %d: %v", p, err)
		}
	}
	for _, p := range []int{0, 80, 1023, 65536} {
		if ValidatePort(p) == nil {
			t.Fatalf("port %d accepted", p)
		}
	}
}

func TestRedirectURLAndMask(t *testing.T) {
	if got := RedirectURL(DefaultRedirectPort); got != "http://127.0.0.1:7420/kite/callback" {
		t.Fatalf("RedirectURL = %q", got)
	}
	if got := MaskKey("kitekey1234abcd"); got != "••••abcd" {
		t.Fatalf("MaskKey = %q", got)
	}
	if got := MaskKey(""); got != "" {
		t.Fatalf("MaskKey(empty) = %q", got)
	}
}

func TestSettingsKeysPassSecretGuard(t *testing.T) {
	ctx := context.Background()
	s, err := settings.Load(ctx, &credBackend{m: map[string]string{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{SettingAPIKey.Name, SettingRedirectPort.Name} {
		if logging.IsSecretLikeKey(n) {
			t.Fatalf("%s looks secret-like", n)
		}
	}
	if err := settings.Set(ctx, s, SettingAPIKey, "kitekey1234abcd"); err != nil {
		t.Fatalf("api key: %v", err)
	}
	if err := settings.Set(ctx, s, SettingRedirectPort, 7421); err != nil {
		t.Fatalf("port: %v", err)
	}
	if err := settings.Set(ctx, s, SettingAPIKey, "bad key"); err == nil {
		t.Fatal("invalid api key accepted")
	}
	for _, n := range settings.Names() {
		raw, err := settings.GetJSON(s, n)
		if err != nil {
			t.Fatal(err)
		}
		if err := settings.SetJSON(ctx, s, n, raw); errors.Is(err, settings.ErrSecretKey) || errors.Is(err, settings.ErrSecretValue) {
			t.Fatalf("registered key %s refused by secret guard: %v", n, err)
		}
	}
}

type credBackend struct{ m map[string]string }

func (b *credBackend) Load(context.Context) (map[string]string, error) { return b.m, nil }
func (b *credBackend) Save(_ context.Context, k, v string) error       { b.m[k] = v; return nil }
