> **P0-2 Time & Market Data Implementation Plan — Task 7.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 7: Kite type aliases, ReadClient interface and error taxonomy

**Files:**
- Create: `internal/kite/types.go`
- Create: `internal/kite/errors.go`
- Test: `internal/kite/errors_test.go`

**Interfaces:**
- Consumes: Task 6: `ratelimit.Priority` (in `HistoricalPicker`). P0-1: `bus.NewTopic`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
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
type HistoricalRequest struct {
	Token      int
	Interval   string
	From, To   time.Time
	Continuous bool
	OI         bool
}
type HistoricalPicker interface {
	HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (HistoricalRequest, bool)) (HistoricalRequest, []HistoricalData, bool, error)
}
var TopicOrderUpdate = bus.NewTopic[Order]("order.update")
type Kind uint8
const (
	KindNone        Kind = iota
	KindSession          // TokenException: session expired, all queues pause, no retry
	KindPermission       // PermissionError: degraded mode, no retry
	KindRateLimited      // HTTP 429: back-off on that class only
	KindNetwork          // NetworkException / transport failure: idempotent GETs retry ≤ 3×
	KindInput            // InputException / DataException (e.g. historical window too large)
	KindOrder            // OrderException
	KindServer           // GeneralException / 5xx: GETs retry once after 2 s
	KindCanceled         // context cancelled or deadline exceeded
	KindUnknown
)
func (k Kind) String() string
type Err struct {
	Kind    Kind
	Op      string // ReadClient method name, e.g. "Historical"
	Code    int    // HTTP status (0 when no response)
	Type    string // Kite error_type
	Message string
	cause   error
}
func (e *Err) Error() string
func (e *Err) Unwrap() error
func IsOffline(err error) bool
func Classify(err error) Kind
```

ARCHITECTURE §5.1 `ReadClient` and §5.3 taxonomy. Type aliases let every other package name Kite types without importing gokiteconnect (rule 3). `HistoricalPicker` lets the backfill choose a window at the moment a historical slot is granted (FR-DATA-003 AC1 across the HTTP layer).

- [ ] **Step 0: Add the Kite dependency**

P0-1 Task 2 already requires gokiteconnect v4.4.3 for the G-0 stub; this is idempotent and confirms the pin.

Run: `GOWORK=off go get github.com/zerodha/gokiteconnect/v4@v4.4.3`
Expected: `go.mod` lists `github.com/zerodha/gokiteconnect/v4 v4.4.3` (with its requirements `github.com/gorilla/websocket v1.4.2`, `github.com/gocarina/gocsv`, `github.com/google/go-querystring`).

- [ ] **Step 1: Write the failing tests**

`internal/kite/errors_test.go`:

```go
package kite

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		err  error
		want Kind
	}{
		{nil, KindNone},
		{kiteconnect.Error{Code: 403, ErrorType: kiteconnect.TokenError}, KindSession},
		{kiteconnect.Error{Code: 403, ErrorType: kiteconnect.PermissionError}, KindPermission},
		{kiteconnect.Error{Code: 429, ErrorType: kiteconnect.NetworkError}, KindRateLimited},
		{kiteconnect.Error{Code: 429, ErrorType: kiteconnect.GeneralError}, KindRateLimited},
		{kiteconnect.NewError(kiteconnect.NetworkError, "Request failed.", nil), KindNetwork},
		{kiteconnect.Error{Code: 400, ErrorType: kiteconnect.InputError}, KindInput},
		{kiteconnect.Error{Code: 400, ErrorType: kiteconnect.DataError}, KindInput},
		{kiteconnect.NewError(kiteconnect.DataError, "Error parsing response.", nil), KindServer},
		{kiteconnect.Error{Code: 400, ErrorType: kiteconnect.OrderError}, KindOrder},
		{kiteconnect.Error{Code: 500, ErrorType: kiteconnect.GeneralError}, KindServer},
		{kiteconnect.Error{Code: 502, ErrorType: ""}, KindServer},
		{context.Canceled, KindCanceled},
		{fmt.Errorf("wrapped: %w", kiteconnect.Error{Code: 403, ErrorType: kiteconnect.TokenError}), KindSession},
		{errors.New("boom"), KindUnknown},
	}
	for i, c := range cases {
		if got := Classify(c.err); got != c.want {
			t.Errorf("case %d (%v): %v, want %v", i, c.err, got, c.want)
		}
	}
}

func TestErrKeepsKiteFieldsAndNoHeaders(t *testing.T) {
	e := wrap("Historical", kiteconnect.Error{Code: 400, ErrorType: kiteconnect.InputError, Message: "interval exceeds max limit: 60 days"})
	if e.Kind != KindInput || e.Code != 400 || e.Op != "Historical" {
		t.Fatalf("%+v", e)
	}
	if got := e.Error(); got != "kite Historical: interval exceeds max limit: 60 days (input, 400 InputException)" {
		t.Fatalf("Error() = %q", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/`
Expected: the build fails — output contains `undefined: Kind` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/kite/types.go`:

```go
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
```

`internal/kite/errors.go`:

```go
package kite

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// Kind is the error taxonomy of ARCHITECTURE §5.3.
type Kind uint8

const (
	KindNone        Kind = iota
	KindSession          // TokenException: session expired, all queues pause, no retry
	KindPermission       // PermissionError: degraded mode, no retry
	KindRateLimited      // HTTP 429: back-off on that class only
	KindNetwork          // NetworkException / transport failure: idempotent GETs retry ≤ 3×
	KindInput            // InputException / DataException (e.g. historical window too large)
	KindOrder            // OrderException
	KindServer           // GeneralException / 5xx: GETs retry once after 2 s
	KindCanceled         // context cancelled or deadline exceeded
	KindUnknown
)

var kindNames = map[Kind]string{
	KindNone: "none", KindSession: "session", KindPermission: "permission", KindRateLimited: "rate-limited",
	KindNetwork: "network", KindInput: "input", KindOrder: "order", KindServer: "server",
	KindCanceled: "canceled", KindUnknown: "unknown",
}

func (k Kind) String() string { return kindNames[k] }

// Err is the error every Client method returns. Message is Kite's text (never a header).
type Err struct {
	Kind    Kind
	Op      string // ReadClient method name, e.g. "Historical"
	Code    int    // HTTP status (0 when no response)
	Type    string // Kite error_type
	Message string
	cause   error
}

func (e *Err) Error() string {
	return fmt.Sprintf("kite %s: %s (%s, %d %s)", e.Op, e.Message, e.Kind, e.Code, e.Type)
}

func (e *Err) Unwrap() error { return e.cause }

// IsOffline reports whether err means Kite could not be reached (NFR-14 offline badge).
func IsOffline(err error) bool { return Classify(err) == KindNetwork }

// Classify maps any error from gokiteconnect, the transport or this package to a Kind.
func Classify(err error) Kind {
	if err == nil {
		return KindNone
	}
	var ke *Err
	if errors.As(err, &ke) {
		return ke.Kind
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return KindCanceled
	}
	var e kiteconnect.Error
	if !errors.As(err, &e) {
		return KindUnknown
	}
	if e.Code == http.StatusTooManyRequests {
		return KindRateLimited
	}
	switch e.ErrorType {
	case kiteconnect.TokenError:
		return KindSession
	case kiteconnect.PermissionError:
		return KindPermission
	case kiteconnect.NetworkError:
		return KindNetwork
	case kiteconnect.OrderError:
		return KindOrder
	case kiteconnect.InputError:
		return KindInput
	case kiteconnect.DataError:
		// gokiteconnect also uses DataException (code 504) for bodies it cannot read or parse
		// (http.go readEnvelope); those are server faults, not window errors.
		if e.Code >= 500 {
			return KindServer
		}
		return KindInput
	}
	if e.Code >= 500 || e.ErrorType == kiteconnect.GeneralError {
		return KindServer
	}
	return KindUnknown
}

func wrap(op string, err error) *Err {
	if err == nil {
		return nil
	}
	var ke *Err
	if errors.As(err, &ke) {
		return ke
	}
	out := &Err{Kind: Classify(err), Op: op, Message: err.Error(), cause: err}
	var e kiteconnect.Error
	if errors.As(err, &e) {
		out.Code, out.Type, out.Message = e.Code, e.ErrorType, e.Message
	}
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/`
Expected: `ok  	tradeloom/internal/kite`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/kite/types.go \
  internal/kite/errors.go \
  internal/kite/errors_test.go
git commit -m "$(cat <<'EOF'
feat(kite): read-only client interface, type aliases and error taxonomy

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
