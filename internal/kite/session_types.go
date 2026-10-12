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
