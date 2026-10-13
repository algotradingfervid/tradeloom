package ticker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/kite"
	"tradeloom/internal/kite/mock"
	"tradeloom/internal/kite/ticker"
)

// Ruling 50: the capability probe (kite.DialTicker) and the ticker's conns both drive the
// gokiteconnect ticker, which writes gorilla's global websocket.DefaultDialer on every dial. They
// must share one process-wide dial lock; -race catches a probe and a conn dialling at once.
func TestProbeAndTickerConnDialConcurrentlyWithoutRace(t *testing.T) {
	srv := mock.NewServer()
	defer srv.Close()
	root := srv.TickerURL()
	const rounds = 3
	for i := 0; i < rounds; i++ {
		c := ticker.KiteDialer("key", "token", &root)(ticker.Events{})
		done := serveAsync(context.Background(), c)
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for j := 0; j < 2; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs <- kite.DialTicker(context.Background(), &root, "key", "token", 2*time.Second)
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: probe dial: %v", i, err)
			}
		}
		waitFor(t, 5*time.Second, "conn connected", func() bool { return srv.Connects() == 3*(i+1) })
		c.Stop()
		waitDone(t, done, 4*time.Second, "Stop")
	}
}
