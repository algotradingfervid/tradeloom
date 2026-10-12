// Package kite is the only gateway to Kite Connect (ARCHITECTURE §2.4 rule 3, §5.1). Other
// packages use the read-only ReadClient interface and the type aliases below, so they never
// import gokiteconnect themselves.
package kite

import (
	"context"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	"github.com/zerodha/gokiteconnect/v4/models"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite/ratelimit"
)

// Aliases of gokiteconnect v4.4.3 types (identical types, importable from this package).
type (
	UserProfile       = kiteconnect.UserProfile
	AllMargins        = kiteconnect.AllMargins
	Instrument        = kiteconnect.Instrument
	Instruments       = kiteconnect.Instruments
	HistoricalData    = kiteconnect.HistoricalData
	Quote             = kiteconnect.Quote
	QuoteLTP          = kiteconnect.QuoteLTP
	QuoteOHLC         = kiteconnect.QuoteOHLC
	Order             = kiteconnect.Order
	Orders            = kiteconnect.Orders
	Trade             = kiteconnect.Trade
	Trades            = kiteconnect.Trades
	Positions         = kiteconnect.Positions
	Holdings          = kiteconnect.Holdings
	AuctionInstrument = kiteconnect.AuctionInstrument
	GTT               = kiteconnect.GTT
	GTTs              = kiteconnect.GTTs
	GetMarginParams   = kiteconnect.GetMarginParams
	OrderMargins      = kiteconnect.OrderMargins
	GetBasketParams   = kiteconnect.GetBasketParams
	BasketMargins     = kiteconnect.BasketMargins
	GetChargesParams  = kiteconnect.GetChargesParams
	OrderCharges      = kiteconnect.OrderCharges
	Alert             = kiteconnect.Alert
	AlertHistory      = kiteconnect.AlertHistory
	// Time is gokiteconnect's timestamp wrapper (HistoricalData.Date, Instrument.Expiry).
	Time = models.Time
)

// ReadClient is every read-only Kite call the app makes. Each call is rate limited at its class
// and queued at ratelimit.PriorityFrom(ctx).
type ReadClient interface {
	Profile(ctx context.Context) (UserProfile, error)
	Margins(ctx context.Context) (AllMargins, error)
	Instruments(ctx context.Context) (Instruments, error)
	Historical(ctx context.Context, token int, interval string, from, to time.Time, continuous, oi bool) ([]HistoricalData, error)
	Quote(ctx context.Context, ids ...string) (Quote, error)
	LTP(ctx context.Context, ids ...string) (QuoteLTP, error)
	OHLC(ctx context.Context, ids ...string) (QuoteOHLC, error)
	Orders(ctx context.Context) (Orders, error)
	OrderHistory(ctx context.Context, orderID string) ([]Order, error)
	Trades(ctx context.Context) (Trades, error)
	OrderTrades(ctx context.Context, orderID string) ([]Trade, error)
	Positions(ctx context.Context) (Positions, error)
	Holdings(ctx context.Context) (Holdings, error)
	AuctionInstruments(ctx context.Context) ([]AuctionInstrument, error)
	GTTs(ctx context.Context) (GTTs, error)
	GTT(ctx context.Context, id int) (GTT, error)
	OrderMargins(ctx context.Context, p GetMarginParams) ([]OrderMargins, error)
	BasketMargins(ctx context.Context, p GetBasketParams) (BasketMargins, error)
	OrderCharges(ctx context.Context, p GetChargesParams) ([]OrderCharges, error)
	Alerts(ctx context.Context, filters map[string]string) ([]Alert, error)
	AlertHistory(ctx context.Context, uuid string) ([]AlertHistory, error)
}

// HistoricalRequest is one GetHistoricalData call. From/To are passed to Kite as IST wall time.
type HistoricalRequest struct {
	Token      int
	Interval   string
	From, To   time.Time
	Continuous bool
	OI         bool
}

// HistoricalPicker lets the backfill service choose which window to fetch at the moment the
// historical class grants a slot, so a window queued later at a higher priority is sent first
// (FR-DATA-003 AC1). pick returns ok=false when there is nothing to fetch.
type HistoricalPicker interface {
	HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (HistoricalRequest, bool)) (HistoricalRequest, []HistoricalData, bool, error)
}

// TopicOrderUpdate carries order updates from the ticker and orders.Sync (§3.4 "order.update").
var TopicOrderUpdate = bus.NewTopic[Order]("order.update")
