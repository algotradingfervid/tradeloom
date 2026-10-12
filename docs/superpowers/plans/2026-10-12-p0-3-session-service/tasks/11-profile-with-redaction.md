> **P0-3 Session & Service Layer Implementation Plan — Task 11.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 11: Profile with redaction

**Files:**
- Create: `internal/kite/profile.go`
- Test: `internal/kite/profile_test.go`

**Interfaces:**
- Consumes: `kiteconnect.UserProfile` (verified `user.go`: `UserID, UserName, UserShortName, AvatarURL, UserType, Email, Broker, Products, OrderTypes, Exchanges`).
- Produces:
  ```go
  type Profile struct{ UserID, Name, ShortName, Email, Broker, AvatarURL, Initials string; Exchanges, Products, OrderTypes []string } // TS kite.Profile
  func ProfileFrom(p kiteconnect.UserProfile) Profile
  func (p Profile) LogValue() slog.Value // logs only the masked user id
  func (s *Session) Profile(ctx context.Context) (Profile, error)
  ```

The avatar URL is kept only if its host is `kite.zerodha.com`. Any other host (Kite avatars are typically object-storage URLs) is dropped and the UI shows initials. Otherwise the webview would fetch from a host outside the NFR-04 allow-list.

- [ ] **Step 1: Write the failing tests**

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run TestProfile -v`
Expected: FAIL to compile with `undefined: ProfileFrom`.

- [ ] **Step 3: Write the implementation**

```go
// internal/kite/profile.go
package kite

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"unicode"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// Profile is the FR-ACCT-001 view model. Read-only by construction (a copy).
type Profile struct {
	UserID     string   `json:"userId"`
	Name       string   `json:"name"`
	ShortName  string   `json:"shortName"`
	Email      string   `json:"email"`
	Broker     string   `json:"broker"`
	AvatarURL  string   `json:"avatarUrl"`
	Initials   string   `json:"initials"`
	Exchanges  []string `json:"exchanges"`
	Products   []string `json:"products"`
	OrderTypes []string `json:"orderTypes"`
}

func ProfileFrom(p kiteconnect.UserProfile) Profile {
	broker := p.Broker
	if strings.EqualFold(broker, "zerodha") || broker == "" {
		broker = "Zerodha"
	}
	return Profile{
		UserID: p.UserID, Name: p.UserName, ShortName: p.UserShortName, Email: p.Email, Broker: broker,
		AvatarURL: allowedAvatar(p.AvatarURL), Initials: initials(p.UserName),
		Exchanges: append([]string(nil), p.Exchanges...), Products: append([]string(nil), p.Products...),
		OrderTypes: append([]string(nil), p.OrderTypes...),
	}
}

// LogValue keeps PII out of logs if a Profile is ever logged.
func (p Profile) LogValue() slog.Value {
	return slog.GroupValue(slog.String("user", MaskUserID(p.UserID)))
}

func allowedAvatar(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "kite.zerodha.com" {
		return ""
	}
	return raw
}

func initials(name string) string {
	var out []rune
	for _, w := range strings.Fields(name) {
		for _, r := range w {
			out = append(out, unicode.ToUpper(r))
			break
		}
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}

// Profile fetches GET /user/profile (FR-ACCT-001).
func (s *Session) Profile(ctx context.Context) (Profile, error) {
	if s.State().State != StateReady {
		return Profile{}, &LoginError{Code: CodeNotLoggedIn, Message: "Log in to Kite to see your profile."}
	}
	p, err := s.d.Profiles.Profile(ctx)
	if err != nil {
		s.HandleTokenError(err)
		msg := "network error"
		var ke kiteconnect.Error
		if errors.As(err, &ke) {
			msg = ke.Message
		}
		return Profile{}, &LoginError{Code: CodeKiteError, Message: "Could not load the profile from Kite: " + msg}
	}
	return ProfileFrom(p), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/kite/ -v`
Expected: PASS (every test in `internal/kite`, including Tasks 1–10).

- [ ] **Step 5: Commit**

```bash
git add internal/kite/profile.go internal/kite/profile_test.go
git commit -m "feat(kite): profile view model with PII-safe logging

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---