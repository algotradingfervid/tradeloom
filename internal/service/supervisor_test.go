package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite"
	"tradeloom/internal/logging"
)

// startRun runs the supervisor and returns a channel closed when Run returns.
func startRun(t *testing.T, e *svcEnv, ctx context.Context) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); e.svc.Run(ctx) }()
	return done
}

func waitDone(t *testing.T, what string, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// settle gives the supervisor time to process published events before asserting that nothing happened.
func settle() { time.Sleep(100 * time.Millisecond) }

// Ruling 61 (1): an out-of-order "ready" published after a logout must not restart Sync; Run trusts State().
func TestRun_StaleReadyAfterLogoutDoesNotStartSync(t *testing.T) {
	e := newSvcEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startRun(t, e, ctx)
	eventually(t, "sync running", func() bool { r, _, _ := e.sync.snapshot(); return r == 1 })
	// The subscription is live: a logout event stops Sync.
	e.session.setState(kite.SessionState{State: kite.StateLoggedOut})
	bus.Publish(e.bus, kite.TopicSessionState, e.session.State())
	eventually(t, "sync stopped", func() bool { r, _, _ := e.sync.snapshot(); return r == 0 })
	// A stale ready(s1), published before the logout but delivered after it, must not restart Sync.
	bus.Publish(e.bus, kite.TopicSessionState, kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s1"})
	settle()
	if r, runs, _ := e.sync.snapshot(); r != 0 || runs != 1 {
		t.Fatalf("stale ready restarted Sync: running=%d runs=%d", r, runs)
	}
	cancel()
	waitDone(t, "Run", done)
}

// Ruling 61 (2): a closed bus closes the subscription; Run stops Sync and returns.
func TestRun_ReturnsWhenBusCloses(t *testing.T) {
	e := newSvcEnv(t)
	done := startRun(t, e, context.Background())
	eventually(t, "sync running", func() bool { r, _, _ := e.sync.snapshot(); return r == 1 })
	e.bus.Close()
	waitDone(t, "Run after bus close", done)
	if r, _, _ := e.sync.snapshot(); r != 0 {
		t.Fatalf("Sync still running after Run returned: %d", r)
	}
}

// Ruling 61 (3): stop() waits for the previous Orders.Run to return, so two runs never overlap.
func TestRun_SlowSyncNeverOverlaps(t *testing.T) {
	e := newSvcEnv(t)
	e.sync.exitDelay = 50 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startRun(t, e, ctx)
	eventually(t, "first run", func() bool { _, runs, _ := e.sync.snapshot(); return runs == 1 })
	for i, id := range []string{"s2", "s3"} {
		e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: id})
		bus.Publish(e.bus, kite.TopicSessionState, e.session.State())
		eventually(t, "restart "+id, func() bool { _, runs, _ := e.sync.snapshot(); return runs == i+2 })
	}
	cancel()
	waitDone(t, "Run", done)
	e.sync.mu.Lock()
	maxRunning := e.sync.maxRunning
	e.sync.mu.Unlock()
	if maxRunning != 1 {
		t.Fatalf("Orders.Run overlapped: %d concurrent runs", maxRunning)
	}
}

func TestRun_NewSessionIDRestartsSync(t *testing.T) {
	e := newSvcEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startRun(t, e, ctx)
	eventually(t, "first run", func() bool { r, runs, _ := e.sync.snapshot(); return r == 1 && runs == 1 })
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanNone, SessionID: "s2"})
	bus.Publish(e.bus, kite.TopicSessionState, e.session.State())
	eventually(t, "restart", func() bool { r, runs, p := e.sync.snapshot(); return r == 1 && runs == 2 && p == kite.PlanNone })
	cancel()
	waitDone(t, "Run", done)
}

func TestRun_NoStartUntilReady(t *testing.T) {
	e := newSvcEnv(t)
	e.session.setState(kite.SessionState{State: kite.StateLoggedOut})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startRun(t, e, ctx)
	settle()
	if _, runs, _ := e.sync.snapshot(); runs != 0 {
		t.Fatalf("Sync started while logged out: %d", runs)
	}
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s9"})
	bus.Publish(e.bus, kite.TopicSessionState, e.session.State())
	eventually(t, "sync after login", func() bool { r, _, _ := e.sync.snapshot(); return r == 1 })
	cancel()
	waitDone(t, "Run", done)
}

func TestRun_ContextCancelStopsSync(t *testing.T) {
	e := newSvcEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := startRun(t, e, ctx)
	eventually(t, "sync running", func() bool { r, _, _ := e.sync.snapshot(); return r == 1 })
	cancel()
	waitDone(t, "Run", done)
	if r, _, _ := e.sync.snapshot(); r != 0 {
		t.Fatalf("Sync still running after cancel: %d", r)
	}
}

// Ruling 61 (4).
func TestNew_PanicsOnMissingDeps(t *testing.T) {
	full := func() Deps {
		e := newSvcEnv(t)
		return e.svc.d
	}
	for _, c := range []struct {
		field string
		clear func(*Deps)
	}{
		{"Session", func(d *Deps) { d.Session = nil }},
		{"Orders", func(d *Deps) { d.Orders = nil }},
		{"Bus", func(d *Deps) { d.Bus = nil }},
		{"Clock", func(d *Deps) { d.Clock = nil }},
	} {
		t.Run(c.field, func(t *testing.T) {
			d := full()
			c.clear(&d)
			defer func() {
				r := recover()
				if r == nil || !strings.Contains(fmtAny(r), c.field) {
					t.Fatalf("panic %v does not name %s", r, c.field)
				}
			}()
			New(d)
		})
	}
}

func fmtAny(v any) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Ruling 61 (5): every Reason is redacted, LoginError messages included.
func TestAsRefused_RedactsRegisteredSecrets(t *testing.T) {
	const secret = "zq9SECRETvalue771"
	logging.RegisterSecret(secret)
	for _, err := range []error{
		&kite.LoginError{Code: kite.CodeKiteError, Message: "rejected " + secret},
		errors.New("boom " + secret),
	} {
		r := asRefused("login", err).(*ErrRefused)
		if strings.Contains(r.Reason, secret) {
			t.Fatalf("secret leaked in Reason %q", r.Reason)
		}
	}
}
