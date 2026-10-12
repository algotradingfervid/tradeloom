package ticker

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	kt "github.com/zerodha/gokiteconnect/v4/ticker"

	"tradeloom/internal/clock"
)

// writeTimeout bounds every Subscribe/Unsubscribe/SetMode frame (Conn's bounded-write contract).
const writeTimeout = 5 * time.Second

var errNotConnected = errors.New("ticker: not connected")

// KiteDialer returns a Dialer over gokiteconnect's ticker. Each Conn is a fresh ticker.Ticker
// with SetReconnectMaxRetries(0): the library then returns from ServeWithContext after the first
// failure instead of reconnecting itself, so the Manager owns reconnects and resubscribes. This
// avoids two library hazards (v4.4.3 ticker.go): Resubscribe mutates the unsynchronised
// subscribedTokens map on the serve goroutine and prints the token list to stdout; and with
// SetAutoReconnect(false) a failed dial dereferences a nil Conn.
//
// The ticker URL carries api_key and access_token in its query: errors passed to ev.OnError have
// both values redacted, and nothing here logs the URL.
func KiteDialer(apiKey, accessToken string, root *url.URL) Dialer {
	return func(ev Events) Conn {
		t := kt.New(apiKey, accessToken)
		if root != nil {
			t.SetRootURL(*root)
		}
		t.SetAutoReconnect(true)
		t.SetReconnectMaxRetries(0)
		t.SetConnectTimeout(7 * time.Second)
		k := &kiteConn{t: t}
		t.OnConnect(func() {
			// Runs on the serve goroutine right after the library assigned t.Conn on it, so reading
			// t.Conn here is race-free; every other goroutine uses k.ws under k.mu.
			ws := t.Conn
			k.mu.Lock()
			k.ws = ws
			closed := k.closed
			k.mu.Unlock()
			if closed { // Stop or ctx cancel raced the dial: never surface a live socket
				_ = ws.Close()
				return
			}
			if ev.OnConnect != nil {
				ev.OnConnect()
			}
		})
		if ev.OnTick != nil {
			t.OnTick(ev.OnTick)
		}
		if ev.OnOrderUpdate != nil {
			t.OnOrderUpdate(ev.OnOrderUpdate)
		}
		if ev.OnError != nil {
			secrets := []string{accessToken, apiKey}
			t.OnError(func(err error) { ev.OnError(redact(err, secrets)) })
		}
		return k
	}
}

// socket is the part of *websocket.Conn kiteConn uses itself (writes go through the library).
type socket interface {
	SetWriteDeadline(t time.Time) error
	Close() error
}

type kiteConn struct {
	t *kt.Ticker

	mu      sync.Mutex
	ws      socket             // set on OnConnect
	cancel  context.CancelFunc // ends the serve ctx; set by Serve
	served  bool
	stopped bool
	closed  bool // serve ctx ended: a socket that connects late is closed at once
}

// Serve runs the library's serve loop under a ctx of its own, so Stop works whether it comes
// before, during or after ServeWithContext (the library's Stop is a no-op until ServeWithContext
// has assigned its cancel, and that assignment is unsynchronised — kiteConn never calls it). When
// the ctx ends (caller cancel, Stop, or a failed write) the socket is closed so the library's
// blocked read returns.
func (k *kiteConn) Serve(ctx context.Context) {
	k.mu.Lock()
	if k.stopped || k.served {
		k.mu.Unlock()
		return
	}
	k.served = true
	ctx, cancel := context.WithCancel(ctx)
	k.cancel = cancel
	k.mu.Unlock()
	defer cancel()
	unregister := context.AfterFunc(ctx, k.closeSocket)
	defer unregister()
	k.t.ServeWithContext(ctx)
}

func (k *kiteConn) closeSocket() {
	k.mu.Lock()
	k.closed = true
	ws := k.ws
	k.mu.Unlock()
	if ws != nil {
		_ = ws.Close()
	}
}

// Stop ends Serve: non-blocking, idempotent, safe before Serve (Serve then returns at once).
func (k *kiteConn) Stop() {
	k.mu.Lock()
	k.stopped = true
	cancel := k.cancel
	k.mu.Unlock()
	if cancel != nil {
		cancel() // AfterFunc closes the socket on its own goroutine
	}
}

func (k *kiteConn) Subscribe(tokens []uint32) error {
	return k.write(func() error { return k.t.Subscribe(tokens) })
}

func (k *kiteConn) Unsubscribe(tokens []uint32) error {
	return k.write(func() error { return k.t.Unsubscribe(tokens) })
}

func (k *kiteConn) SetMode(m Mode, tokens []uint32) error {
	return k.write(func() error { return k.t.SetMode(m.kite(), tokens) })
}

// write runs one library write under a writeTimeout deadline. Writes are never concurrent (Conn
// contract) and k.mu is not held while writing, so Stop never waits on a stalled socket. A failed
// write leaves the WebSocket unusable, so it ends Serve and the Manager redials and resubscribes.
func (k *kiteConn) write(op func() error) error {
	k.mu.Lock()
	ws, closed, cancel := k.ws, k.closed || k.stopped, k.cancel
	k.mu.Unlock()
	if ws == nil || closed {
		return errNotConnected
	}
	err := ws.SetWriteDeadline(clock.SystemNow().Add(writeTimeout))
	if err == nil {
		err = op()
	}
	if err != nil && cancel != nil {
		cancel()
	}
	return err
}

// redact removes every non-empty secret from err's text.
func redact(err error, secrets []string) error {
	if err == nil {
		return nil
	}
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
