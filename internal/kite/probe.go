package kite

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	kiteticker "github.com/zerodha/gokiteconnect/v4/ticker"

	"tradeloom/internal/clock"
	"tradeloom/internal/logging"
)

// NiftyIndexToken is the NIFTY 50 index instrument token in Kite's instrument dump (S0-1 re-checks it).
const NiftyIndexToken = 256265

const probeTickerTimeout = 5 * time.Second

type HistoricalReader interface {
	Historical(ctx context.Context, token int, iv string, from, to time.Time, continuous, oi bool) ([]kiteconnect.HistoricalData, error)
}

// CapabilityProbe decides Connect vs Personal plan without placing anything (FR-ACCT-007).
type CapabilityProbe struct {
	Historical    func(ctx context.Context) error
	TickerConnect func(ctx context.Context, apiKey, accessToken string) error
	Log           *slog.Logger
}

func NewCapabilityProbe(h HistoricalReader, clk clock.Clock, tickerRoot *url.URL, log *slog.Logger) *CapabilityProbe {
	return &CapabilityProbe{
		Historical: func(ctx context.Context) error {
			now := clk.Now()
			_, err := h.Historical(ctx, NiftyIndexToken, "day", now.AddDate(0, 0, -7), now, false, false)
			return err
		},
		TickerConnect: func(ctx context.Context, apiKey, accessToken string) error {
			return DialTicker(ctx, tickerRoot, apiKey, accessToken, probeTickerTimeout)
		},
		Log: log,
	}
}

// Probe implements the decision table of ARCH §5.2. Logs carry only the error kind, never an
// error text (a transport error may hold the ticker URL with its credentials).
func (p *CapabilityProbe) Probe(ctx context.Context, apiKey, accessToken string) DataPlan {
	err := p.Historical(ctx)
	switch {
	case err == nil:
		return PlanConnect
	case IsNoDataPermission(err):
		p.Log.Info("capability probe: no historical-data permission", "kind", Classify(err).String())
		return PlanNone
	}
	p.Log.Warn("capability probe: historical request inconclusive; trying the ticker", "kind", Classify(err).String())
	terr := p.TickerConnect(ctx, apiKey, accessToken)
	if terr == nil {
		return PlanConnect
	}
	p.Log.Warn("capability probe: ticker handshake failed", "kind", Classify(terr).String())
	return PlanUnknown
}

// IsNoDataPermission recognises every observed spelling of "this API key has no market-data permission".
func IsNoDataPermission(err error) bool {
	var ke kiteconnect.Error
	if !errors.As(err, &ke) {
		return false
	}
	if ke.ErrorType == kiteconnect.TokenError {
		return false
	}
	if ke.ErrorType == kiteconnect.PermissionError || ke.ErrorType == "PermissionException" || ke.Code == 403 {
		return true
	}
	return strings.Contains(strings.ToLower(ke.Message), "permission")
}

// dialSem is the process-wide gokiteconnect dial lock (Ruling 50, P0-2 Ruling 46).
var dialSem = make(chan struct{}, 1)

// AcquireDial takes the process-wide lock that every gokiteconnect ticker dial must hold. The
// library's ServeWithContext writes gorilla's global websocket.DefaultDialer on each dial (v4.4.3
// ticker.go:299–300), a data race when two tickers (the Manager's slots, the capability probe)
// dial at once. Hold it from just before ServeWithContext until OnConnect fires or
// ServeWithContext returns, whichever is first. Waiting ends with ctx's error when ctx ends, so a
// Stop never waits behind another dial. The returned release is idempotent.
func AcquireDial(ctx context.Context) (release func(), err error) {
	select {
	case dialSem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var once sync.Once
	return func() { once.Do(func() { <-dialSem }) }, nil
}

// DialTicker opens one WebSocket connection and closes it again. It returns nil when the handshake succeeded.
//
// It applies the discipline of ticker.KiteDialer to the library's hazards: the dial holds
// AcquireDial; the serve loop runs under a ctx of its own that DialTicker cancels itself (the
// library's Stop is unsynchronised with ServeWithContext's cancel assignment, so it is never
// called); the socket is closed on the serve goroutine as soon as OnConnect fires, so a caller
// cancel racing the handshake never leaves a live socket; and every returned error has the API
// key and access token (raw and query-escaped) redacted, since the ticker URL carries both.
func DialTicker(ctx context.Context, root *url.URL, apiKey, accessToken string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout+2*time.Second)
	defer cancel()
	tk := kiteticker.New(apiKey, accessToken)
	if root != nil {
		tk.SetRootURL(*root)
	}
	// Auto-reconnect with zero retries: in v4.4.3, ServeWithContext with auto-reconnect off falls
	// through a failed Dial into OnConnect and a nil-Conn SetCloseHandler (false success, then panic).
	tk.SetAutoReconnect(true)
	tk.SetReconnectMaxRetries(0)
	tk.SetConnectTimeout(timeout)

	secrets := []string{accessToken, apiKey, url.QueryEscape(accessToken), url.QueryEscape(apiKey)}
	var (
		mu          sync.Mutex
		lastErr     error
		releaseDial func()
	)
	res := make(chan error, 1)
	report := func(err error) {
		select {
		case res <- err:
		default:
		}
	}
	tk.OnError(func(err error) {
		mu.Lock()
		if lastErr == nil {
			lastErr = err // the dial error; later ones come from closing the probe socket
		}
		mu.Unlock()
	})
	tk.OnConnect(func() {
		// Runs on the serve goroutine right after the library assigned tk.Conn on it: race-free.
		conn := tk.Conn
		if conn != nil {
			_ = conn.Close()
		}
		report(nil)
		cancel() // the serve loop returns at its next ctx check and never dials again
		mu.Lock()
		release := releaseDial
		mu.Unlock()
		if release != nil {
			release() // the dial is over: let the next dialer proceed
		}
	})
	tk.OnNoReconnect(func(int) {
		mu.Lock()
		err := lastErr
		mu.Unlock()
		if err == nil {
			err = errors.New("ticker connection failed")
		}
		report(fmt.Errorf("ticker connect: %w", redactSecrets(err, secrets)))
	})
	logging.Go("kite.probe.ticker", func() {
		release, err := AcquireDial(ctx)
		if err != nil {
			return
		}
		defer release()
		mu.Lock()
		releaseDial = release
		mu.Unlock()
		tk.ServeWithContext(ctx)
	})
	select {
	case err := <-res:
		return err
	case <-ctx.Done():
		select {
		case err := <-res: // the outcome landed together with the ctx end (OnConnect cancels ctx)
			return err
		default:
		}
		return fmt.Errorf("ticker connect: %w", ctx.Err())
	}
}

// redactSecrets removes every non-empty secret from err's text.
func redactSecrets(err error, secrets []string) error {
	msg := err.Error()
	out := msg
	for _, s := range secrets {
		if s != "" {
			out = strings.ReplaceAll(out, s, "[redacted]")
		}
	}
	if out == msg {
		return err
	}
	return errors.New(out)
}
