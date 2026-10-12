// internal/service/session_test.go
package service

import (
	"context"
	"errors"
	"testing"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite"
)

func TestAsRefused_MapsLoginError(t *testing.T) {
	err := asRefused("login", &kite.LoginError{Code: kite.CodeRequestTokenInvalid, Message: "Click again"})
	var r *ErrRefused
	if !errors.As(err, &r) || r.Code != kite.CodeRequestTokenInvalid || r.Stage != "login" || r.Reason != "Click again" {
		t.Fatalf("%#v", err)
	}
	if asRefused("x", nil) != nil {
		t.Fatal("nil must stay nil")
	}
	if got := asRefused("x", errors.New("boom")).(*ErrRefused); got.Code != "internal" {
		t.Fatalf("%#v", got)
	}
}

func TestSessionStatus_MergesCredentials(t *testing.T) {
	e := newSvcEnv(t)
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanNone, ExpiresAtMs: 42, RiskAckRequired: true})
	st := e.svc.SessionStatus(context.Background())
	if st.State != "ready" || st.DataPlan != "none" || st.ExpiresAtMs != 42 || !st.RiskAckRequired || st.Credentials.APIKeyMasked != "••••abcd" {
		t.Fatalf("%+v", st)
	}
}

func TestBeginLogin_ErrorIsRefused(t *testing.T) {
	e := newSvcEnv(t)
	e.session.err = &kite.LoginError{Code: kite.CodeNoCredentials, Message: "Set up first"}
	_, err := e.svc.BeginLogin(context.Background())
	if refusedCode(t, err) != kite.CodeNoCredentials {
		t.Fatal(err)
	}
}

// Review Focus 3: after expiry orders.Sync stops, so nothing polls Kite until the next login.
func TestRun_StopsSyncWhenSessionExpires(t *testing.T) {
	e := newSvcEnv(t)
	// Ready before Run starts: Run applies the current state right after subscribing, so no event can be missed.
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s1"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.svc.Run(ctx)
	eventually(t, "sync running", func() bool { r, _, p := e.sync.snapshot(); return r == 1 && p == kite.PlanConnect })
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanNone, SessionID: "s1"})
	eventually(t, "plan update", func() bool { _, _, p := e.sync.snapshot(); return p == kite.PlanNone })
	if _, runs, _ := e.sync.snapshot(); runs != 1 {
		t.Fatalf("sync restarted %d times for the same session", runs)
	}
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateExpired, ErrorCode: kite.CodeSessionExpired})
	eventually(t, "sync stopped", func() bool { r, _, _ := e.sync.snapshot(); return r == 0 })
}

// *kite.Session (Tasks 6–11) satisfies SessionAPI; cmd/tradeloom wires it as Deps.Session.
var _ SessionAPI = (*kite.Session)(nil)

// Ruling 52: a secret-store failure keeps its own code; it is never reported as no_credentials.
func TestAsRefused_KeepsSecretStoreCode(t *testing.T) {
	err := asRefused("credentials", &kite.LoginError{Code: kite.CodeSecretStore, Message: "secret store not answering"})
	if r := err.(*ErrRefused); r.Code != kite.CodeSecretStore {
		t.Fatalf("%#v", r)
	}
}
