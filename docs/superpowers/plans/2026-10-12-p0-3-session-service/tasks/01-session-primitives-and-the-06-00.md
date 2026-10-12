> **P0-3 Session & Service Layer Implementation Plan — Task 1.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 1: Session primitives and the 06:00 IST expiry rule

**Files:**
- Create: `internal/kite/session_types.go`, `internal/kite/expiry.go`
- Test: `internal/kite/expiry_test.go`

**Interfaces:**
- Consumes: `bus.NewTopic`, `clock.IST`, `kiteconnect.Error` (verified `errors.go`: `Error{Code int; ErrorType, Message string; Data interface{}}`, constants `TokenError="TokenException"`, `PermissionError="PermissionError"`, `InputError`, `NetworkError`).
- Produces:
  ```go
  type State string // StateNoCredentials "no_credentials", StateLoggedOut "logged_out", StateLoggingIn "logging_in", StateReady "ready", StateExpired "expired"
  type DataPlan string // PlanUnknown "unknown", PlanConnect "connect", PlanNone "none"
  type SessionState struct{ State State; SessionID, UserID string; LoggedInAtMs, ExpiresAtMs int64; DataPlan DataPlan; Offline, RiskAckRequired bool; ErrorCode, ErrorMessage string }
  type TokenRecord struct{ AccessToken, UserID, SessionID string; LoggedInAtMs, ExpiresAtMs int64 }
  type LoginError struct{ Code, Message string } // implements error
  var TopicSessionState bus.Topic[SessionState] // "session.state"
  func NextExpiry(t time.Time) time.Time
  func APIError(errType string, httpCode int, msg string) error // builds a kiteconnect.Error (for tests in packages that may not import gokiteconnect)
  func MaskUserID(id string) string
  // codes: CodeNoCredentials, CodeInvalidCredentials, CodeCredentialsRejected, CodeAlreadyLoggedIn, CodeLoginBusy, CodeBadRedirect,
  // CodeLoginCancelled, CodeStateMismatch, CodeNotPending, CodeRequestTokenInvalid, CodeListenerTimeout, CodeNetwork, CodeKiteError,
  // CodeSecretStore, CodeSessionExpired, CodeTokenRejected, CodeNotLoggedIn
  ```

- [ ] **Step 1: Write the failing test**

```go
// internal/kite/expiry_test.go
package kite

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/clock"
)

func ist(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, clock.IST)
}

func TestNextExpiry(t *testing.T) {
	cases := []struct {
		name     string
		in, want time.Time
	}{
		{"morning login expires next day 06:00", ist(2026, 10, 12, 9, 2), ist(2026, 10, 13, 6, 0)},
		{"login at 05:59 expires the same day 06:00", ist(2026, 10, 12, 5, 59), ist(2026, 10, 12, 6, 0)},
		{"login exactly at 06:00 expires next day", ist(2026, 10, 12, 6, 0), ist(2026, 10, 13, 6, 0)},
		{"late night", ist(2026, 10, 12, 23, 59), ist(2026, 10, 13, 6, 0)},
		{"UTC input is converted to IST", time.Date(2026, 10, 12, 3, 32, 0, 0, time.UTC), ist(2026, 10, 13, 6, 0)},
		{"month end", ist(2026, 10, 31, 10, 0), ist(2026, 11, 1, 6, 0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NextExpiry(c.in); !got.Equal(c.want) {
				t.Fatalf("NextExpiry(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestSessionStateJSONIsBindingFriendly(t *testing.T) {
	b, err := json.Marshal(SessionState{State: StateReady, ExpiresAtMs: 1, DataPlan: PlanConnect})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"state":"ready"`, `"expiresAtMs":1`, `"dataPlan":"connect"`, `"riskAckRequired":false`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("%s missing in %s", want, b)
		}
	}
}

func TestMaskUserID(t *testing.T) {
	if got := MaskUserID("AB1234"); got != "AB****" {
		t.Fatalf("got %q", got)
	}
	if got := MaskUserID("A"); got != "*" {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestNextExpiry|TestSessionStateJSON|TestMaskUserID' -v`
Expected: FAIL to compile with `undefined: NextExpiry`, `undefined: SessionState`, `undefined: MaskUserID`.

- [ ] **Step 3: Write the implementation**

```go
// internal/kite/session_types.go
package kite

import (
	"strings"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/bus"
)

// State is the broker-session state behind the status indicator (FR-SHELL-005) and the login views.
type State string

const (
	StateNoCredentials State = "no_credentials"
	StateLoggedOut     State = "logged_out"
	StateLoggingIn     State = "logging_in"
	StateReady         State = "ready"
	StateExpired       State = "expired"
)

// DataPlan is the result of the capability probe (FR-ACCT-007).
type DataPlan string

const (
	PlanUnknown DataPlan = "unknown"
	PlanConnect DataPlan = "connect"
	PlanNone    DataPlan = "none"
)

// Codes carried by SessionState.ErrorCode and LoginError.Code.
const (
	CodeNoCredentials       = "no_credentials"
	CodeInvalidCredentials  = "invalid_credentials"
	CodeCredentialsRejected = "credentials_rejected"
	CodeAlreadyLoggedIn     = "already_logged_in"
	CodeLoginBusy           = "login_in_progress"
	CodeBadRedirect         = "bad_redirect"
	CodeLoginCancelled      = "login_cancelled"
	CodeStateMismatch       = "state_mismatch"
	CodeNotPending          = "login_not_pending"
	CodeRequestTokenInvalid = "request_token_invalid"
	CodeListenerTimeout     = "listener_timeout"
	CodeNetwork             = "network"
	CodeKiteError           = "kite_error"
	CodeSecretStore         = "secret_store"
	CodeSessionExpired      = "session_expired"
	CodeTokenRejected       = "token_rejected"
	CodeNotLoggedIn         = "not_logged_in"
)

// SessionState is published on TopicSessionState on every transition. JSON-friendly (§8.4).
type SessionState struct {
	State           State    `json:"state"`
	SessionID       string   `json:"sessionId"`
	UserID          string   `json:"userId"`
	LoggedInAtMs    int64    `json:"loggedInAtMs"`
	ExpiresAtMs     int64    `json:"expiresAtMs"`
	DataPlan        DataPlan `json:"dataPlan"`
	Offline         bool     `json:"offline"`
	RiskAckRequired bool     `json:"riskAckRequired"`
	ErrorCode       string   `json:"errorCode"`
	ErrorMessage    string   `json:"errorMessage"`
}

// TokenRecord is stored as JSON under secrets.KeyAccessToken (never in app.db).
type TokenRecord struct {
	AccessToken  string `json:"accessToken"`
	UserID       string `json:"userId"`
	SessionID    string `json:"sessionId"`
	LoggedInAtMs int64  `json:"loggedInAtMs"`
	ExpiresAtMs  int64  `json:"expiresAtMs"`
}

// LoginError is a user-facing session error; Message never contains a secret or token.
type LoginError struct {
	Code    string
	Message string
}

func (e *LoginError) Error() string { return e.Message }

// TopicSessionState mirrors the Wails event "session.state" (ARCH §3.4).
var TopicSessionState = bus.NewTopic[SessionState]("session.state")

// APIError builds a Kite API error value. Packages that must not import gokiteconnect (rule 3) use it in tests.
func APIError(errType string, httpCode int, msg string) error {
	return kiteconnect.Error{Code: httpCode, ErrorType: errType, Message: msg}
}

// MaskUserID keeps the first two characters of a Kite client id for logs ("AB1234" -> "AB****").
func MaskUserID(id string) string {
	if len(id) <= 2 {
		return strings.Repeat("*", len(id))
	}
	return id[:2] + strings.Repeat("*", len(id)-2)
}
```

```go
// internal/kite/expiry.go
package kite

import (
	"time"

	"tradeloom/internal/clock"
)

// NextExpiry returns the first 06:00 IST strictly after t (A07 §A2: the token expires at 6 AM the next day;
// a login before 06:00 expires the same morning).
func NextExpiry(t time.Time) time.Time {
	in := t.In(clock.IST)
	six := time.Date(in.Year(), in.Month(), in.Day(), 6, 0, 0, 0, clock.IST)
	if !in.Before(six) {
		six = six.AddDate(0, 0, 1)
	}
	return six
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestNextExpiry|TestSessionStateJSON|TestMaskUserID' -v`
Expected: PASS (8 subtests and 2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/kite/session_types.go internal/kite/expiry.go internal/kite/expiry_test.go
git commit -m "feat(kite): session state types and 06:00 IST expiry rule

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
