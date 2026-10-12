// Package ticker is the WebSocket subscription manager (C-07, ARCHITECTURE §5.6): up to three
// gokiteconnect tickers × 3000 tokens, consumer priorities, own reconnect with resubscribe,
// mode switching, 100 ms coalesced fan-out on the bus, raw per-tick streams, clock samples.
package ticker

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zerodha/gokiteconnect/v4/models"
	kt "github.com/zerodha/gokiteconnect/v4/ticker"

	"tradeloom/internal/bus"
)

// Mode is a subscription mode; higher includes lower (ltp < quote < full).
type Mode uint8

const (
	ModeLTP   Mode = 1
	ModeQuote Mode = 2
	ModeFull  Mode = 3 // the only mode carrying the exchange timestamp (clock samples)
)

func (m Mode) String() string {
	switch m {
	case ModeLTP:
		return "ltp"
	case ModeQuote:
		return "quote"
	case ModeFull:
		return "full"
	}
	return fmt.Sprintf("mode(%d)", m)
}

func (m Mode) kite() kt.Mode {
	switch m {
	case ModeLTP:
		return kt.ModeLTP
	case ModeFull:
		return kt.ModeFull
	}
	return kt.ModeQuote
}

// Consumer is a class of subscriber, highest priority first.
type Consumer uint8

const (
	ConsumerClock Consumer = iota // the clock reference token (manager-owned)
	ConsumerDeployment
	ConsumerLiveScanner
	ConsumerChart
	ConsumerWatchlist
	ConsumerWidget
	ConsumerRecorder
)

var consumerNames = []string{"clock", "deployment", "live-scanner", "chart", "watchlist", "widget", "recorder"}

func (c Consumer) String() string {
	if int(c) < len(consumerNames) {
		return consumerNames[c]
	}
	return fmt.Sprintf("consumer(%d)", c)
}

// Tick is the app's tick (JSON for the UI bridge). Prices in rupees.
type Tick struct {
	Token         uint32    `json:"token"`
	Mode          string    `json:"mode"`
	LTP           float64   `json:"ltp"`
	Change        float64   `json:"change"` // LTP − previous close (Kite net_change)
	Open          float64   `json:"open"`
	High          float64   `json:"high"`
	Low           float64   `json:"low"`
	Close         float64   `json:"close"`  // previous session close
	Volume        uint32    `json:"volume"` // cumulative day volume
	LastQty       uint32    `json:"lastQty"`
	OI            uint32    `json:"oi"`
	ExchangeTS    time.Time `json:"exchangeTs"`    // zero unless full mode
	LastTradeTime time.Time `json:"lastTradeTime"` // zero unless full mode (equities)
	Received      time.Time `json:"received"`      // app clock at arrival
	IsIndex       bool      `json:"isIndex"`
}

// fromKite converts gokiteconnect's tick.
func fromKite(t models.Tick, received time.Time) Tick {
	return Tick{
		Token: t.InstrumentToken, Mode: t.Mode, LTP: t.LastPrice, Change: t.NetChange,
		Open: t.OHLC.Open, High: t.OHLC.High, Low: t.OHLC.Low, Close: t.OHLC.Close,
		Volume: t.VolumeTraded, LastQty: t.LastTradedQuantity, OI: t.OI,
		ExchangeTS: t.Timestamp.Time, LastTradeTime: t.LastTradeTime.Time, Received: received, IsIndex: t.IsIndex,
	}
}

// ErrBudget refuses a subscription that would exceed the 3 × 3000 token budget (C-07).
type ErrBudget struct {
	Used, Requested, Limit int
	ByConsumer             map[string]int
}

func (e *ErrBudget) Error() string {
	var parts []string
	for k, v := range e.ByConsumer {
		parts = append(parts, fmt.Sprintf("%s %d", k, v))
	}
	sort.Strings(parts)
	return fmt.Sprintf("live-data budget exceeded: %d in use + %d new > %d (%s)", e.Used, e.Requested, e.Limit, strings.Join(parts, ", "))
}

// FeedState is "feed.state" (§3.4): live | connecting | reconnecting | offline | stopped.
type FeedState struct {
	State       string    `json:"state"`
	Attempt     int       `json:"attempt"`
	NextRetry   time.Time `json:"nextRetry"`
	LastTick    time.Time `json:"lastTick"`
	Connections int       `json:"connections"`
	Tokens      int       `json:"tokens"`
}

// Usage is the budget breakdown for Settings → Limits (FR-ACCT-009).
type Usage struct {
	Used        int            `json:"used"`
	Limit       int            `json:"limit"`
	Connections int            `json:"connections"`
	ByConsumer  map[string]int `json:"byConsumer"`
	RawDrops    uint64         `json:"rawDrops"`
}

var (
	// TopicTicks is "tick.batch": latest tick per token, at most every Coalesce interval.
	TopicTicks = bus.NewTopic[[]Tick]("tick.batch")
	// TopicFeed is "feed.state".
	TopicFeed = bus.NewTopic[FeedState]("feed.state")
)
