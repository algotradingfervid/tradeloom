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

// Conn is one WebSocket connection. Serve blocks until the connection ends; Subscribe,
// Unsubscribe and SetMode are only called after OnConnect and never concurrently.
type Conn interface {
	Serve(ctx context.Context)
	Subscribe(tokens []uint32) error
	Unsubscribe(tokens []uint32) error
	SetMode(m Mode, tokens []uint32) error
	Stop()
}

// Dialer creates a Conn wired to ev.
type Dialer func(ev Events) Conn
