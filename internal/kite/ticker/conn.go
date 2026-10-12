package ticker

import (
	"context"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	"github.com/zerodha/gokiteconnect/v4/models"
)

// Events are the callbacks a Conn invokes (on its own goroutine).
type Events struct {
	OnConnect     func()
	OnTick        func(models.Tick)
	OnOrderUpdate func(kiteconnect.Order)
	OnError       func(error)
}

// Conn is one WebSocket connection. Serve blocks until the connection ends or ctx is done; Subscribe,
// Unsubscribe and SetMode are only called after OnConnect and never concurrently.
//
// The Manager calls Subscribe, Unsubscribe and SetMode while holding its registry lock, so every
// write MUST be bounded (a write deadline, ~5 s for the Kite connection): an unbounded write on a
// stalled socket would block every Subscribe, Usage and reconnect until the socket died. A write
// that times out returns an error; the Manager logs it and the reconnect resubscribes in full.
// Stop must be safe to call at any time, including before Serve connects and more than once.
type Conn interface {
	Serve(ctx context.Context)
	Subscribe(tokens []uint32) error
	Unsubscribe(tokens []uint32) error
	SetMode(m Mode, tokens []uint32) error
	Stop()
}

// Dialer creates a Conn wired to ev.
type Dialer func(ev Events) Conn
