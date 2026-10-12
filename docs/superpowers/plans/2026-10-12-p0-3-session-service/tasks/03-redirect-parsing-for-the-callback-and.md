> **P0-3 Session & Service Layer Implementation Plan — Task 3.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 3: Redirect parsing for the callback and the paste fallback

**Files:**
- Create: `internal/kite/redirect.go`
- Test: `internal/kite/redirect_test.go`

**Interfaces:**
- Consumes: `LoginError`, the codes from Task 1.
- Produces: `type Redirect struct{ RequestToken, State, Status string }`; `func ParseRedirect(input string) (Redirect, error)`. It accepts a full URL, a URL without a scheme, a bare query string or a bare token. It rejects `status` values other than `success` with `CodeLoginCancelled`, and missing or malformed tokens with `CodeBadRedirect`.

Kite's docs promise only `request_token` on the redirect (A07 §A2, re-read 2026-10-12). `status`/`action` are community-observed, so `status` is checked only when present. `state` comes back because we send it in `redirect_params` (Kite docs: "sent back to the redirect URL").

- [ ] **Step 1: Write the failing test**

```go
// internal/kite/redirect_test.go
package kite

import (
	"errors"
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
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRedirect(c.in)
			var le *LoginError
			if !errors.As(err, &le) || le.Code != c.code {
				t.Fatalf("ParseRedirect(%q) = %v, want code %s", c.in, err, c.code)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run TestParseRedirect -v`
Expected: FAIL to compile with `undefined: ParseRedirect`.

- [ ] **Step 3: Write the implementation**

```go
// internal/kite/redirect.go
package kite

import (
	"net/url"
	"regexp"
	"strings"
)

// Redirect is what Kite sends back to the redirect URL after a login.
type Redirect struct {
	RequestToken string
	State        string
	Status       string
}

var requestTokenRe = regexp.MustCompile(`^[A-Za-z0-9]{8,128}$`)

func badRedirect(msg string) error { return &LoginError{Code: CodeBadRedirect, Message: msg} }

// ParseRedirect extracts the request_token from a callback query, a pasted URL, a pasted query or a bare token.
func ParseRedirect(input string) (Redirect, error) {
	in := strings.Trim(strings.TrimSpace(input), "\"'<>")
	if in == "" {
		return Redirect{}, badRedirect("Paste the full address from the browser after logging in, or just the request_token.")
	}
	if requestTokenRe.MatchString(in) {
		return Redirect{RequestToken: in}, nil
	}
	q := in
	if i := strings.IndexByte(q, '?'); i >= 0 {
		q = q[i+1:]
	}
	if i := strings.IndexByte(q, '#'); i >= 0 {
		q = q[:i]
	}
	vals, err := url.ParseQuery(q)
	if err != nil {
		return Redirect{}, badRedirect("That address could not be read. Copy the whole address bar after logging in.")
	}
	r := Redirect{
		RequestToken: strings.TrimSpace(vals.Get("request_token")),
		State:        vals.Get("state"),
		Status:       vals.Get("status"),
	}
	if r.Status != "" && r.Status != "success" {
		return Redirect{}, &LoginError{Code: CodeLoginCancelled, Message: "Kite reports the login was not completed (status \"" + r.Status + "\"). Click \"Log in with Kite\" to try again."}
	}
	if !requestTokenRe.MatchString(r.RequestToken) {
		return Redirect{}, badRedirect("No request_token found in the pasted text. After logging in, copy the whole address from the browser's address bar.")
	}
	return r, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run TestParseRedirect -v`
Expected: PASS (14 subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/kite/redirect.go internal/kite/redirect_test.go
git commit -m "feat(kite): parse login redirects for loopback and paste fallback

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
