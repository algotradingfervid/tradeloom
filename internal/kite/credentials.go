// internal/kite/credentials.go
package kite

import (
	"fmt"
	"regexp"
	"strings"

	"tradeloom/internal/settings"
)

// CallbackPath is the loopback redirect path registered in the Kite developer console (ARCH §5.2).
const CallbackPath = "/kite/callback"

// DefaultRedirectPort is the default loopback port (Q-A3 decided: 7420, user-configurable).
const DefaultRedirectPort = 7420

var credRe = regexp.MustCompile(`^[A-Za-z0-9]{6,64}$`)

// Settings keys of this package (P0-1 Key[T] literals, registered for the generic façade of Task 17).
var (
	SettingAPIKey = settings.Register(settings.Key[string]{Name: "kite.api_key", Default: "", Validate: func(v string) error {
		if v == "" {
			return nil
		}
		return ValidateAPIKey(v)
	}})
	SettingRedirectPort = settings.Register(settings.Key[int]{Name: "kite.redirect_port", Default: DefaultRedirectPort, Validate: ValidatePort})
)

// NormalizeCredential removes whitespace, newlines and quotes picked up when copying from a web page.
func NormalizeCredential(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'`")
	return strings.TrimSpace(s)
}

func ValidateAPIKey(k string) error {
	if !credRe.MatchString(k) {
		return &LoginError{Code: CodeInvalidCredentials, Message: "The API key should be 6–64 letters and digits. Copy it from your app in the Kite developer console."}
	}
	return nil
}

func ValidateAPISecret(s string) error {
	if !credRe.MatchString(s) {
		return &LoginError{Code: CodeInvalidCredentials, Message: "The API secret should be 6–64 letters and digits. Copy it from your app in the Kite developer console."}
	}
	return nil
}

func ValidatePort(p int) error {
	if p < 1024 || p > 65535 {
		return fmt.Errorf("redirect port must be between 1024 and 65535, got %d", p)
	}
	return nil
}

// RedirectURL is the exact URL the user registers as the app's redirect URL (FR-ACCT-005).
func RedirectURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, CallbackPath)
}

// MaskKey shows only the last four characters of the API key (FR-ACCT-005).
func MaskKey(k string) string {
	if len(k) <= 4 {
		return strings.Repeat("•", len(k))
	}
	return "••••" + k[len(k)-4:]
}

// CredentialsInfo is what Settings → Kite connection shows. It never carries the secret.
type CredentialsInfo struct {
	Configured    bool   `json:"configured"`
	APIKeyMasked  string `json:"apiKeyMasked"`
	Port          int    `json:"port"`
	RedirectURL   string `json:"redirectUrl"`
	SecretBackend string `json:"secretBackend"`
}
