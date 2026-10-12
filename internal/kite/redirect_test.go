// internal/kite/redirect_test.go
package kite

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRedirect(t *testing.T) {
	const tok = "AbC123xyz789"
	ok := []struct {
		name, in, state string
	}{
		{"full callback URL", "http://127.0.0.1:7420/kite/callback?request_token=" + tok + "&action=login&status=success&state=n1", "n1"},
		{"no scheme", "127.0.0.1:7420/kite/callback?request_token=" + tok + "&status=success", ""},
		{"bare token", tok, ""},
		{"quotes and whitespace", "  \"http://127.0.0.1:7420/kite/callback?request_token=" + tok + "\"\n", ""},
		{"query only", "request_token=" + tok + "&status=success", ""},
		{"leading question mark", "?request_token=" + tok, ""},
		{"other host or port", "https://example.org/cb?request_token=" + tok, ""},
		{"fragment ignored", "http://127.0.0.1:7420/kite/callback?request_token=" + tok + "#top", ""},
	}
	for _, c := range ok {
		t.Run(c.name, func(t *testing.T) {
			r, err := ParseRedirect(c.in)
			if err != nil {
				t.Fatalf("ParseRedirect(%q) error %v", c.in, err)
			}
			if r.RequestToken != tok || r.State != c.state {
				t.Fatalf("got %+v", r)
			}
		})
	}

	bad := []struct {
		name, in, code string
	}{
		{"empty", "   ", CodeBadRedirect},
		{"no token", "http://127.0.0.1:7420/kite/callback?status=success", CodeBadRedirect},
		{"prose", "hello world", CodeBadRedirect},
		{"too short", "abc", CodeBadRedirect},
		{"token with symbols", "request_token=abc$%^123456", CodeBadRedirect},
		{"cancelled", "http://127.0.0.1:7420/kite/callback?request_token=" + tok + "&status=cancelled", CodeLoginCancelled},
		{"foreign scheme", "javascript://x/cb?request_token=" + tok, CodeBadRedirect},
		{"file scheme", "file:///tmp/cb?request_token=" + tok, CodeBadRedirect},
		{"oversize", "request_token=" + tok + "&x=" + strings.Repeat("a", 5000), CodeBadRedirect},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRedirect(c.in)
			var le *LoginError
			if !errors.As(err, &le) || le.Code != c.code {
				t.Fatalf("ParseRedirect(%q) = %v, want code %s", c.in, err, c.code)
			}
			if strings.Contains(err.Error(), tok) {
				t.Fatalf("error leaks token: %v", err)
			}
		})
	}
}
