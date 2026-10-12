// Package ratelimit is the shared Kite rate limiter (FR-DATA-003, ARCHITECTURE §5.4): one
// sliding-window admission per endpoint class, priority queues with FIFO inside a priority,
// per-class 429 back-off, global pause on session expiry, and 1 Hz usage events.
package ratelimit

import (
	"context"
	"fmt"
)

// Class is a Kite endpoint class with its own rate (A07 §A3).
type Class uint8

const (
	ClassQuote      Class = iota // /quote, /quote/ltp, /quote/ohlc — 1 req/s
	ClassHistorical              // /instruments/historical — 3 req/s
	ClassOrders                  // order placement — 10 req/s at Kite; app default 8 (FR-ORD-006)
	ClassOther                   // everything else — 10 req/s
	numClasses
)

var classNames = [numClasses]string{"quote", "historical", "orders", "other"}

func (c Class) String() string {
	if c < numClasses {
		return classNames[c]
	}
	return fmt.Sprintf("class(%d)", c)
}

// Priority orders requests inside a class; lower value is served first (FR-DATA-003).
type Priority uint8

const (
	Interactive Priority = iota
	Deployment
	LiveScanner
	Backtest
	Recorder
	Library
	NumPriorities
)

var prioNames = [NumPriorities]string{"interactive", "deployment", "live-scanner", "backtest", "recorder", "library"}

func (p Priority) String() string {
	if p < NumPriorities {
		return prioNames[p]
	}
	return fmt.Sprintf("priority(%d)", p)
}

// DefaultRates are requests per second per class.
var DefaultRates = map[Class]int{ClassQuote: 1, ClassHistorical: 3, ClassOrders: 8, ClassOther: 10}

type ctxKey struct{}

// WithPriority tags ctx so kite.Client calls made with it queue at p.
func WithPriority(ctx context.Context, p Priority) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// PriorityFrom returns the priority carried by ctx; Interactive when none.
func PriorityFrom(ctx context.Context) Priority {
	if p, ok := ctx.Value(ctxKey{}).(Priority); ok {
		return p
	}
	return Interactive
}
