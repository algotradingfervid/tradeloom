package service

import (
	"context"
	"testing"
	"time"

	"tradeloom/internal/candles"
	"tradeloom/internal/kite/ticker"
)

// Ruling 66 → Task 23: a frontend reload and the shutdown close every live chart subscription.
func TestCloseAllLive_ReleasesEveryChart(t *testing.T) {
	e := newSvcEnv(t)
	_, _, tr := withChartDeps(t, e)
	ctx := context.Background()
	for _, tok := range []uint32{256265, 256265, 10} {
		if _, err := e.svc.OpenLive(ctx, tok, "5minute"); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.svc.CloseAllLive(ctx); n != 3 {
		t.Fatalf("closed %d", n)
	}
	if _, un := tr.counts(256265, candles.Minute5); un != 1 || e.live.has(ticker.ConsumerChart, 256265) || e.live.has(ticker.ConsumerChart, 10) {
		t.Fatalf("untracked %d", un)
	}
	if n := e.svc.CloseAllLive(ctx); n != 0 {
		t.Fatalf("second close %d", n)
	}
}

// Task 23 (Ruling 74 (9), Task 30 M9): Wait returns only after every tracked service goroutine ended.
func TestWait_BlocksUntilTrackedGoroutinesEnd(t *testing.T) {
	e := newSvcEnv(t)
	hold := make(chan struct{})
	e.svc.goTracked("test.hold", func() { <-hold })
	done := make(chan struct{})
	go func() { e.svc.Wait(); close(done) }()
	select {
	case <-done:
		t.Fatal("Wait returned while a goroutine was running")
	case <-time.After(100 * time.Millisecond):
	}
	close(hold)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not return")
	}
	e.svc.Wait() // idle: returns at once
}

// Ruling 57 → Task 23: the forwarded (argv) redirect goes to the session's callback-rule path.
func TestCompleteForwardedLogin_UsesCallbackRule(t *testing.T) {
	e := newSvcEnv(t)
	u := "http://127.0.0.1:7420/kite/callback?request_token=rtok12345678&state=abc"
	if _, err := e.svc.CompleteForwardedLogin(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	e.session.mu.Lock()
	defer e.session.mu.Unlock()
	if len(e.session.forwarded) != 1 || e.session.forwarded[0] != u || len(e.session.completed) != 0 {
		t.Fatalf("forwarded %v completed %v", e.session.forwarded, e.session.completed)
	}
}
