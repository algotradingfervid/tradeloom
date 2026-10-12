// internal/service/lifecycle.go
package service

import (
	"context"
	"sync"
	"time"

	"tradeloom/internal/logging"
)

// goGroup counts the service's own background goroutines (jobs, event sources, market clock, linger
// timers) so the shutdown can wait for them before app.db closes (Task 23; Ruling 74 (9), Task 30 M9).
// Unlike sync.WaitGroup, starting a goroutine while Wait runs is allowed.
type goGroup struct {
	mu   sync.Mutex
	n    int
	idle *sync.Cond
}

func (g *goGroup) add() {
	g.mu.Lock()
	g.n++
	g.mu.Unlock()
}

func (g *goGroup) done() {
	g.mu.Lock()
	g.n--
	if g.n == 0 && g.idle != nil {
		g.idle.Broadcast()
	}
	g.mu.Unlock()
}

func (g *goGroup) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.n
}

func (g *goGroup) wait() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.idle == nil {
		g.idle = sync.NewCond(&g.mu)
	}
	for g.n > 0 {
		g.idle.Wait()
	}
}

// goTracked starts fn through logging.Go and counts it for Wait. Goroutines that may block without a
// bound (the keyring calls of boundedKeyring) are deliberately not tracked.
func (s *Service) goTracked(name string, fn func()) {
	s.bg.add()
	logging.Go(name, func() {
		defer s.bg.done()
		fn()
	})
}

// ShutdownJobWait bounds Wait: a running backup or support-bundle job gets this long to finish at shutdown.
const ShutdownJobWait = 30 * time.Second

// Wait blocks until every tracked service goroutine has returned (a running backup or support-bundle job
// finishes; event sources, the market clock and linger timers end once the App context is cancelled and
// RunQuotes has returned), but never longer than ShutdownJobWait: after that it logs a Warn and returns,
// so a wedged goroutine cannot hang the exit. cmd/tradeloom calls it from App.Deps.StopPlatform, after
// App.Shutdown cancelled the App context and before boot.Shutdown closes app.db.
func (s *Service) Wait() { s.waitFor(ShutdownJobWait) }

// waitFor is Wait with an explicit bound; it reports whether every tracked goroutine ended in time.
func (s *Service) waitFor(d time.Duration) bool {
	done := make(chan struct{})
	logging.Go("service.wait", func() {
		s.bg.wait()
		close(done)
	})
	// A context deadline on the wait (not app time; Ruling 10), like App.Shutdown's bounded wait.
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		s.d.Log.Warn("shutdown: service goroutines still running; continuing", "waited", d.String(), "running", s.bg.count())
		return false
	}
}

// CloseAllLive closes every open live-chart subscription (Ruling 66): on a frontend reload (the page that
// opened them is gone) and at shutdown. It returns how many were closed.
func (s *Service) CloseAllLive(ctx context.Context) int {
	s.chartMu.Lock()
	ids := make([]string, 0, len(s.liveCharts))
	for id := range s.liveCharts {
		ids = append(ids, id)
	}
	s.chartMu.Unlock()
	n := 0
	for _, id := range ids {
		if s.CloseLive(ctx, id) == nil {
			n++
		}
	}
	return n
}

// CompleteForwardedLogin completes a login from a Kite redirect URL handed over by a second launch
// (internal/app.SecondInstance). It follows the loopback callback rule, not the paste rule (Ruling 57):
// the URL must carry the state nonce of the current login attempt.
func (s *Service) CompleteForwardedLogin(ctx context.Context, redirectURL string) (SessionStatus, error) {
	st, err := s.d.Session.CompleteForwardedLogin(ctx, redirectURL)
	return s.statusFrom(st), s.refuse("login", err)
}
