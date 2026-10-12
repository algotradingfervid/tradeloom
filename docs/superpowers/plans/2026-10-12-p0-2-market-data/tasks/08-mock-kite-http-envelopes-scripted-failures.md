> **P0-2 Time & Market Data Implementation Plan — Task 8.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 8: Mock Kite: HTTP envelopes, scripted failures, binary ticks, in-memory fake

**Files:**
- Create: `internal/kite/mock/packets.go`
- Create: `internal/kite/mock/server.go`
- Create: `internal/kite/mock/fake.go`
- Test: `internal/kite/mock/mock_test.go`

**Interfaces:**
- Consumes: Task 1: `clock.IST`. Task 6: `ratelimit.Limiter`, `ratelimit.ClassHistorical`, `ratelimit.Priority`. Task 7: `kite.ReadClient`, `kite.HistoricalPicker`, `kite.HistoricalRequest`, all aliases. P0-1: `logging.Go`.
- Produces (exact Go signatures; later tasks and P0-3/P0-5 copy them):

```go
type Packet struct {
	Token         uint32
	Mode          string // "ltp", "quote" or "full"
	LastPrice     float64
	LastQty       uint32
	AvgPrice      float64
	Volume        uint32
	BuyQty        uint32
	SellQty       uint32
	Open          float64
	High          float64
	Low           float64
	Close         float64
	LastTradeTime time.Time
	OI            uint32
	Timestamp     time.Time // exchange timestamp; full mode only
}
func (p Packet) Encode() []byte
func Frame(ps ...Packet) []byte
type Failure struct {
	Status    int
	ErrorType string // e.g. "TokenException", "NetworkException", "InputException"
	Message   string
}
type Candle struct {
	T                      time.Time
	Open, High, Low, Close float64
	Volume                 int64
	OI                     int64
}
type HistReq struct {
	Token    int
	Interval string
	From, To string // as sent: "2006-01-02 15:04:05"
	OI       string
}
const (
	RouteHistorical  = "historical"
	RouteInstruments = "instruments"
	RouteQuote       = "quote"
	RouteLTP         = "ltp"
	RouteProfile     = "profile"
	RouteOrders      = "orders"
	RouteOrderWrite  = "order-write" // any POST/PUT/DELETE under /orders or /gtt: must stay 0 in P0–P3
)
type Server struct {
	*httptest.Server

	mu         sync.Mutex
	failures   map[string][]Failure
	calls      map[string]int
	hist       []HistReq
	candles    map[string][]Candle // key token/interval
	dump       []byte
	date       func() time.Time
	conns      []*websocket.Conn
	connects   int
	wsMessages []string
	upgrader   websocket.Upgrader
	rejectWS   int // number of upcoming WS handshakes to refuse
}
func NewServer() *Server
func (s *Server) SetDate(f func() time.Time)
func (s *Server) SetInstrumentsCSV(b []byte)
func (s *Server) SetCandles(token int, interval string, cs []Candle)
func (s *Server) FailNext(route string, f Failure)
func (s *Server) Calls(route string) int
func (s *Server) HistoricalRequests() []HistReq
func (s *Server) TickerURL() url.URL
func (s *Server) RejectNextHandshakes(n int)
func (s *Server) Connects() int
func (s *Server) Messages() []string
func (s *Server) PushTicks(ps ...Packet)
func (s *Server) PushOrderUpdate(orderJSON string)
func (s *Server) DropConnections()
type Fake struct {
	mu sync.Mutex
	// HistoricalFunc answers Historical and HistoricalPick; nil returns no candles.
	HistoricalFunc func(req kite.HistoricalRequest) ([]kite.HistoricalData, error)
	// InstrumentsFunc answers Instruments; nil returns an empty dump.
	InstrumentsFunc func() (kite.Instruments, error)
	// Limiter, when set, makes HistoricalPick wait for a historical slot like the real client.
	Limiter *ratelimit.Limiter
	calls   map[string]int
	hist    []kite.HistoricalRequest
}
func NewFake() *Fake
func (f *Fake) Calls(m string) int
func (f *Fake) HistoricalRequests() []kite.HistoricalRequest
func (f *Fake) Historical(ctx context.Context, token int, iv string, from, to time.Time, continuous, oi bool) ([]kite.HistoricalData, error)
func (f *Fake) HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (kite.HistoricalRequest, bool)) (kite.HistoricalRequest, []kite.HistoricalData, bool, error)
func (f *Fake) Instruments(ctx context.Context) (kite.Instruments, error)
func (f *Fake) Profile(ctx context.Context) (kite.UserProfile, error)
func (f *Fake) Margins(ctx context.Context) (kite.AllMargins, error)
func (f *Fake) Quote(ctx context.Context, ids ...string) (kite.Quote, error)
func (f *Fake) LTP(ctx context.Context, ids ...string) (kite.QuoteLTP, error)
func (f *Fake) OHLC(ctx context.Context, ids ...string) (kite.QuoteOHLC, error)
func (f *Fake) Orders(ctx context.Context) (kite.Orders, error)
func (f *Fake) OrderHistory(ctx context.Context, id string) ([]kite.Order, error)
func (f *Fake) Trades(ctx context.Context) (kite.Trades, error)
func (f *Fake) OrderTrades(ctx context.Context, id string) ([]kite.Trade, error)
func (f *Fake) Positions(ctx context.Context) (kite.Positions, error)
func (f *Fake) Holdings(ctx context.Context) (kite.Holdings, error)
func (f *Fake) AuctionInstruments(ctx context.Context) ([]kite.AuctionInstrument, error)
func (f *Fake) GTTs(ctx context.Context) (kite.GTTs, error)
func (f *Fake) GTT(ctx context.Context, id int) (kite.GTT, error)
func (f *Fake) OrderMargins(ctx context.Context, p kite.GetMarginParams) ([]kite.OrderMargins, error)
func (f *Fake) BasketMargins(ctx context.Context, p kite.GetBasketParams) (kite.BasketMargins, error)
func (f *Fake) OrderCharges(ctx context.Context, p kite.GetChargesParams) ([]kite.OrderCharges, error)
func (f *Fake) Alerts(ctx context.Context, filters map[string]string) ([]kite.Alert, error)
func (f *Fake) AlertHistory(ctx context.Context, uuid string) ([]kite.AlertHistory, error)
func (f *Fake) Methods() []string
```

ARCHITECTURE §11 "mock Kite HTTP + WebSocket server": every later Kite-facing test runs against it. Packets follow the layouts `ticker.parsePacket` decodes (8/28/32/44/184 bytes); order-write routes are recorded so G-0 check 3 can assert zero calls.

- [ ] **Step 1: Write the failing tests**

`internal/kite/mock/mock_test.go`:

```go
package mock

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
)

func TestPacketSizesMatchKiteModes(t *testing.T) {
	cases := []struct {
		p    Packet
		size int
	}{
		{Packet{Token: 738561, Mode: "ltp"}, 8},
		{Packet{Token: 256265, Mode: "quote"}, 28}, // 256265 & 0xFF == 9: index
		{Packet{Token: 256265, Mode: "full"}, 32},
		{Packet{Token: 738561, Mode: "quote"}, 44},
		{Packet{Token: 738561, Mode: "full"}, 184},
	}
	for _, c := range cases {
		if n := len(c.p.Encode()); n != c.size {
			t.Errorf("%d/%s: %d bytes, want %d", c.p.Token, c.p.Mode, n, c.size)
		}
	}
	ts := time.Date(2026, 10, 12, 10, 0, 1, 0, clock.IST)
	b := Packet{Token: 738561, Mode: "full", LastPrice: 2450.05, Timestamp: ts}.Encode()
	if binary.BigEndian.Uint32(b[4:8]) != 245005 || binary.BigEndian.Uint32(b[60:64]) != uint32(ts.Unix()) {
		t.Fatal("price or exchange timestamp misplaced")
	}
	f := Frame(Packet{Token: 1, Mode: "ltp"}, Packet{Token: 2, Mode: "ltp"})
	if binary.BigEndian.Uint16(f[0:2]) != 2 || binary.BigEndian.Uint16(f[2:4]) != 8 || len(f) != 2+2*(2+8) {
		t.Fatalf("frame % x", f)
	}
}

func TestServerScriptsFailuresAndCountsCalls(t *testing.T) {
	s := NewServer()
	defer s.Close()
	s.FailNext(RouteProfile, Failure{Status: 403, ErrorType: "TokenException", Message: "expired"})
	resp, err := http.Get(s.URL + "/user/profile")
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	if resp.StatusCode != 403 || env["error_type"] != "TokenException" || env["status"] != "error" {
		t.Fatalf("status %d env %v", resp.StatusCode, env)
	}
	resp, _ = http.Get(s.URL + "/user/profile")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != `{"status":"success","data":{"user_id":"AB1234","user_name":"Test User"}}` {
		t.Fatalf("second call %d %s", resp.StatusCode, body)
	}
	if s.Calls(RouteProfile) != 2 {
		t.Fatalf("calls = %d", s.Calls(RouteProfile))
	}
	req, _ := http.NewRequest(http.MethodPost, s.URL+"/orders/regular", nil)
	if resp, err := http.DefaultClient.Do(req); err == nil {
		resp.Body.Close()
	}
	if s.Calls(RouteOrderWrite) != 1 {
		t.Fatal("order-write route not recorded")
	}
}

func TestServerFiltersHistoricalByRange(t *testing.T) {
	s := NewServer()
	defer s.Close()
	d := func(day int) time.Time { return time.Date(2026, 10, day, 0, 0, 0, 0, clock.IST) }
	s.SetCandles(5, "day", []Candle{{T: d(8), Close: 1}, {T: d(9), Close: 2}, {T: d(12), Close: 3}})
	resp, err := http.Get(s.URL + "/instruments/historical/5/day?from=2026-10-09+00:00:00&to=2026-10-11+23:59:59&oi=0")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Data struct{ Candles [][]any } `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	if len(env.Data.Candles) != 1 || env.Data.Candles[0][0] != "2026-10-09T00:00:00+0530" {
		t.Fatalf("candles %v", env.Data.Candles)
	}
	if r := s.HistoricalRequests(); len(r) != 1 || r[0].From != "2026-10-09 00:00:00" {
		t.Fatalf("recorded %+v", r)
	}
}

func TestFakeCountsAndPicks(t *testing.T) {
	f := NewFake()
	var rc kite.ReadClient = f
	_, _ = rc.Orders(context.Background())
	req, _, ok, err := f.HistoricalPick(context.Background(), 0, func() (kite.HistoricalRequest, bool) {
		return kite.HistoricalRequest{Token: 9, Interval: "day"}, true
	})
	if !ok || err != nil || req.Token != 9 || f.Calls("Historical") != 1 || f.Calls("Orders") != 1 {
		t.Fatalf("pick %+v %v %v calls %v", req, ok, err, f.Methods())
	}
	if _, _, ok, _ := f.HistoricalPick(context.Background(), 0, func() (kite.HistoricalRequest, bool) { return kite.HistoricalRequest{}, false }); ok {
		t.Fatal("empty pick reported ok")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/mock/`
Expected: the build fails — output contains `undefined: Packet` and `[build failed]`.

- [ ] **Step 3: Write the implementation**

`internal/kite/mock/packets.go`:

```go
package mock

import (
	"encoding/binary"
	"math"
	"time"
)

// Packet is one tick in Kite's binary WebSocket format (A07 §A6; parsed by gokiteconnect
// ticker.parsePacket). Prices are rupees; the encoder writes paise. Token&0xFF == 9 marks an index.
type Packet struct {
	Token         uint32
	Mode          string // "ltp", "quote" or "full"
	LastPrice     float64
	LastQty       uint32
	AvgPrice      float64
	Volume        uint32
	BuyQty        uint32
	SellQty       uint32
	Open          float64
	High          float64
	Low           float64
	Close         float64
	LastTradeTime time.Time
	OI            uint32
	Timestamp     time.Time // exchange timestamp; full mode only
}

func paise(v float64) uint32 { return uint32(math.Round(v * 100)) }

func unix(t time.Time) uint32 {
	if t.IsZero() {
		return 0
	}
	return uint32(t.Unix())
}

// Encode returns the packet bytes: ltp 8; index quote 28 / full 32; equity quote 44 / full 184.
func (p Packet) Encode() []byte {
	be := binary.BigEndian
	if p.Mode == "ltp" {
		b := make([]byte, 8)
		be.PutUint32(b[0:], p.Token)
		be.PutUint32(b[4:], paise(p.LastPrice))
		return b
	}
	if p.Token&0xFF == 9 { // index
		n := 28
		if p.Mode == "full" {
			n = 32
		}
		b := make([]byte, n)
		be.PutUint32(b[0:], p.Token)
		be.PutUint32(b[4:], paise(p.LastPrice))
		be.PutUint32(b[8:], paise(p.High))
		be.PutUint32(b[12:], paise(p.Low))
		be.PutUint32(b[16:], paise(p.Open))
		be.PutUint32(b[20:], paise(p.Close))
		be.PutUint32(b[24:], paise(p.LastPrice-p.Close))
		if n == 32 {
			be.PutUint32(b[28:], unix(p.Timestamp))
		}
		return b
	}
	n := 44
	if p.Mode == "full" {
		n = 184
	}
	b := make([]byte, n)
	be.PutUint32(b[0:], p.Token)
	be.PutUint32(b[4:], paise(p.LastPrice))
	be.PutUint32(b[8:], p.LastQty)
	be.PutUint32(b[12:], paise(p.AvgPrice))
	be.PutUint32(b[16:], p.Volume)
	be.PutUint32(b[20:], p.BuyQty)
	be.PutUint32(b[24:], p.SellQty)
	be.PutUint32(b[28:], paise(p.Open))
	be.PutUint32(b[32:], paise(p.High))
	be.PutUint32(b[36:], paise(p.Low))
	be.PutUint32(b[40:], paise(p.Close))
	if n == 184 {
		be.PutUint32(b[44:], unix(p.LastTradeTime))
		be.PutUint32(b[48:], p.OI)
		be.PutUint32(b[60:], unix(p.Timestamp))
	}
	return b
}

// Frame packs packets into one binary WebSocket message: uint16 count, then per packet a
// uint16 length followed by the packet.
func Frame(ps ...Packet) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, uint16(len(ps)))
	for _, p := range ps {
		b := p.Encode()
		l := make([]byte, 2)
		binary.BigEndian.PutUint16(l, uint16(len(b)))
		out = append(out, l...)
		out = append(out, b...)
	}
	return out
}
```

`internal/kite/mock/server.go`:

```go
// Package mock is the in-process Kite used by tests (ARCHITECTURE §11): Server speaks Kite's
// HTTP envelope and binary WebSocket formats on httptest; Fake implements kite.ReadClient in memory.
package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"tradeloom/internal/clock"
	"tradeloom/internal/logging"
)

// Failure is a scripted Kite error response.
type Failure struct {
	Status    int
	ErrorType string // e.g. "TokenException", "NetworkException", "InputException"
	Message   string
}

// Candle is one historical row as Kite returns it.
type Candle struct {
	T                      time.Time
	Open, High, Low, Close float64
	Volume                 int64
	OI                     int64
}

// HistReq records one historical request the server received.
type HistReq struct {
	Token    int
	Interval string
	From, To string // as sent: "2006-01-02 15:04:05"
	OI       string
}

// Route names for FailNext and Calls.
const (
	RouteHistorical  = "historical"
	RouteInstruments = "instruments"
	RouteQuote       = "quote"
	RouteLTP         = "ltp"
	RouteProfile     = "profile"
	RouteOrders      = "orders"
	RouteOrderWrite  = "order-write" // any POST/PUT/DELETE under /orders or /gtt: must stay 0 in P0–P3
)

// Server is a mock Kite HTTP + WebSocket endpoint.
type Server struct {
	*httptest.Server

	mu         sync.Mutex
	failures   map[string][]Failure
	calls      map[string]int
	hist       []HistReq
	candles    map[string][]Candle // key token/interval
	dump       []byte
	date       func() time.Time
	conns      []*websocket.Conn
	connects   int
	wsMessages []string
	upgrader   websocket.Upgrader
	rejectWS   int // number of upcoming WS handshakes to refuse
}

// NewServer starts a mock server. Close it with s.Close().
func NewServer() *Server {
	s := &Server{failures: map[string][]Failure{}, calls: map[string]int{}, candles: map[string][]Candle{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/instruments/historical/", s.historical)
	mux.HandleFunc("/instruments", s.instruments)
	mux.HandleFunc("/quote", s.simple(RouteQuote, `{}`))
	mux.HandleFunc("/quote/ltp", s.simple(RouteLTP, `{}`))
	mux.HandleFunc("/user/profile", s.simple(RouteProfile, `{"user_id":"AB1234","user_name":"Test User"}`))
	mux.HandleFunc("/orders", s.orders)
	mux.HandleFunc("/orders/", s.orders)
	mux.HandleFunc("/gtt/", s.orders)
	mux.HandleFunc("/ws", s.ws)
	s.Server = httptest.NewServer(mux)
	return s
}

// SetDate makes every response carry Date = f() (default: real time).
func (s *Server) SetDate(f func() time.Time) { s.mu.Lock(); s.date = f; s.mu.Unlock() }

// SetInstrumentsCSV sets the /instruments body (plain CSV with Kite's header).
func (s *Server) SetInstrumentsCSV(b []byte) { s.mu.Lock(); s.dump = b; s.mu.Unlock() }

// SetCandles stores the candles served for (token, interval); requests are filtered by from/to.
func (s *Server) SetCandles(token int, interval string, cs []Candle) {
	s.mu.Lock()
	s.candles[fmt.Sprintf("%d/%s", token, interval)] = cs
	s.mu.Unlock()
}

// FailNext queues a failure for the next request on route.
func (s *Server) FailNext(route string, f Failure) {
	s.mu.Lock()
	s.failures[route] = append(s.failures[route], f)
	s.mu.Unlock()
}

// Calls returns how many requests route received (including failed ones).
func (s *Server) Calls(route string) int { s.mu.Lock(); defer s.mu.Unlock(); return s.calls[route] }

// HistoricalRequests returns the historical requests received, in order.
func (s *Server) HistoricalRequests() []HistReq {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]HistReq(nil), s.hist...)
}

func (s *Server) begin(w http.ResponseWriter, route string) bool {
	s.mu.Lock()
	s.calls[route]++
	if s.date != nil {
		w.Header().Set("Date", s.date().UTC().Format(http.TimeFormat))
	}
	var f *Failure
	if q := s.failures[route]; len(q) > 0 {
		f = &q[0]
		s.failures[route] = q[1:]
	}
	s.mu.Unlock()
	if f != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.Status)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "error_type": f.ErrorType, "message": f.Message})
		return false
	}
	return true
}

func ok(w http.ResponseWriter, data string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"success","data":%s}`, data)
}

func (s *Server) simple(route, data string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.begin(w, route) {
			ok(w, data)
		}
	}
}

func (s *Server) orders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.begin(w, RouteOrderWrite)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if s.begin(w, RouteOrders) {
		ok(w, `[]`)
	}
}

func (s *Server) instruments(w http.ResponseWriter, r *http.Request) {
	if !s.begin(w, RouteInstruments) {
		return
	}
	s.mu.Lock()
	b := s.dump
	s.mu.Unlock()
	w.Header().Set("Content-Type", "text/csv")
	_, _ = w.Write(b)
}

func (s *Server) historical(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/instruments/historical/"), "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	token, _ := strconv.Atoi(parts[0])
	q := r.URL.Query()
	s.mu.Lock()
	s.hist = append(s.hist, HistReq{Token: token, Interval: parts[1], From: q.Get("from"), To: q.Get("to"), OI: q.Get("oi")})
	cs := s.candles[fmt.Sprintf("%d/%s", token, parts[1])]
	s.mu.Unlock()
	if !s.begin(w, RouteHistorical) {
		return
	}
	from, _ := time.ParseInLocation("2006-01-02 15:04:05", q.Get("from"), clock.IST)
	to, _ := time.ParseInLocation("2006-01-02 15:04:05", q.Get("to"), clock.IST)
	rows := make([][]any, 0)
	for _, c := range cs {
		if c.T.Before(from) || c.T.After(to) {
			continue
		}
		row := []any{c.T.In(clock.IST).Format("2006-01-02T15:04:05-0700"), c.Open, c.High, c.Low, c.Close, c.Volume}
		if q.Get("oi") == "1" {
			row = append(row, c.OI)
		}
		rows = append(rows, row)
	}
	b, _ := json.Marshal(map[string]any{"candles": rows})
	ok(w, string(b))
}

// TickerURL is the ws:// URL to pass to ticker.Ticker.SetRootURL.
func (s *Server) TickerURL() url.URL {
	u, _ := url.Parse(s.URL)
	return url.URL{Scheme: "ws", Host: u.Host, Path: "/ws"}
}

// RejectNextHandshakes makes the next n WebSocket handshakes fail with HTTP 503.
func (s *Server) RejectNextHandshakes(n int) { s.mu.Lock(); s.rejectWS += n; s.mu.Unlock() }

func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.rejectWS > 0 {
		s.rejectWS--
		s.mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	s.mu.Unlock()
	c, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.conns = append(s.conns, c)
	s.connects++
	s.mu.Unlock()
	logging.Go("mock.ws.read", func() {
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.wsMessages = append(s.wsMessages, string(msg))
			s.mu.Unlock()
		}
	})
}

// Connects returns how many WebSocket connections were accepted.
func (s *Server) Connects() int { s.mu.Lock(); defer s.mu.Unlock(); return s.connects }

// Messages returns the text frames clients sent (subscribe/mode/unsubscribe JSON).
func (s *Server) Messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.wsMessages...)
}

// PushTicks sends one binary frame with the packets to every open connection.
func (s *Server) PushTicks(ps ...Packet) {
	frame := Frame(ps...)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		_ = c.WriteMessage(websocket.BinaryMessage, frame)
	}
}

// PushOrderUpdate sends an order-update text frame (Kite: {"type":"order","data":{…}}).
func (s *Server) PushOrderUpdate(orderJSON string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"type":"order","data":`+orderJSON+`}`))
	}
}

// DropConnections closes every WebSocket connection abruptly (reconnect tests).
func (s *Server) DropConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		_ = c.Close()
	}
	s.conns = nil
}
```

`internal/kite/mock/fake.go`:

```go
package mock

import (
	"context"
	"sort"
	"sync"
	"time"

	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ratelimit"
)

// Fake is an in-memory kite.ReadClient and kite.HistoricalPicker. Unscripted methods return
// zero values. Every call is counted by method name.
type Fake struct {
	mu sync.Mutex
	// HistoricalFunc answers Historical and HistoricalPick; nil returns no candles.
	HistoricalFunc func(req kite.HistoricalRequest) ([]kite.HistoricalData, error)
	// InstrumentsFunc answers Instruments; nil returns an empty dump.
	InstrumentsFunc func() (kite.Instruments, error)
	// Limiter, when set, makes HistoricalPick wait for a historical slot like the real client.
	Limiter *ratelimit.Limiter
	calls   map[string]int
	hist    []kite.HistoricalRequest
}

var _ kite.ReadClient = (*Fake)(nil)
var _ kite.HistoricalPicker = (*Fake)(nil)

// NewFake returns an empty Fake.
func NewFake() *Fake { return &Fake{calls: map[string]int{}} }

func (f *Fake) count(m string) { f.mu.Lock(); f.calls[m]++; f.mu.Unlock() }

// Calls returns how many times method m was called.
func (f *Fake) Calls(m string) int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls[m] }

// HistoricalRequests returns historical requests in call order.
func (f *Fake) HistoricalRequests() []kite.HistoricalRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]kite.HistoricalRequest(nil), f.hist...)
}

func (f *Fake) historical(req kite.HistoricalRequest) ([]kite.HistoricalData, error) {
	f.mu.Lock()
	f.calls["Historical"]++
	f.hist = append(f.hist, req)
	fn := f.HistoricalFunc
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(req)
}

func (f *Fake) Historical(ctx context.Context, token int, iv string, from, to time.Time, continuous, oi bool) ([]kite.HistoricalData, error) {
	return f.historical(kite.HistoricalRequest{Token: token, Interval: iv, From: from, To: to, Continuous: continuous, OI: oi})
}

func (f *Fake) HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (kite.HistoricalRequest, bool)) (kite.HistoricalRequest, []kite.HistoricalData, bool, error) {
	if f.Limiter != nil {
		if err := f.Limiter.Acquire(ctx, ratelimit.ClassHistorical, prio); err != nil {
			return kite.HistoricalRequest{}, nil, false, err
		}
	}
	req, ok := pick()
	if !ok {
		return kite.HistoricalRequest{}, nil, false, nil
	}
	data, err := f.historical(req)
	return req, data, true, err
}

func (f *Fake) Instruments(ctx context.Context) (kite.Instruments, error) {
	f.count("Instruments")
	if f.InstrumentsFunc == nil {
		return nil, nil
	}
	return f.InstrumentsFunc()
}

func (f *Fake) Profile(ctx context.Context) (kite.UserProfile, error) {
	f.count("Profile")
	return kite.UserProfile{}, nil
}
func (f *Fake) Margins(ctx context.Context) (kite.AllMargins, error) {
	f.count("Margins")
	return kite.AllMargins{}, nil
}
func (f *Fake) Quote(ctx context.Context, ids ...string) (kite.Quote, error) {
	f.count("Quote")
	return kite.Quote{}, nil
}
func (f *Fake) LTP(ctx context.Context, ids ...string) (kite.QuoteLTP, error) {
	f.count("LTP")
	return kite.QuoteLTP{}, nil
}
func (f *Fake) OHLC(ctx context.Context, ids ...string) (kite.QuoteOHLC, error) {
	f.count("OHLC")
	return kite.QuoteOHLC{}, nil
}
func (f *Fake) Orders(ctx context.Context) (kite.Orders, error) { f.count("Orders"); return nil, nil }
func (f *Fake) OrderHistory(ctx context.Context, id string) ([]kite.Order, error) {
	f.count("OrderHistory")
	return nil, nil
}
func (f *Fake) Trades(ctx context.Context) (kite.Trades, error) { f.count("Trades"); return nil, nil }
func (f *Fake) OrderTrades(ctx context.Context, id string) ([]kite.Trade, error) {
	f.count("OrderTrades")
	return nil, nil
}
func (f *Fake) Positions(ctx context.Context) (kite.Positions, error) {
	f.count("Positions")
	return kite.Positions{}, nil
}
func (f *Fake) Holdings(ctx context.Context) (kite.Holdings, error) {
	f.count("Holdings")
	return nil, nil
}
func (f *Fake) AuctionInstruments(ctx context.Context) ([]kite.AuctionInstrument, error) {
	f.count("AuctionInstruments")
	return nil, nil
}
func (f *Fake) GTTs(ctx context.Context) (kite.GTTs, error) { f.count("GTTs"); return nil, nil }
func (f *Fake) GTT(ctx context.Context, id int) (kite.GTT, error) {
	f.count("GTT")
	return kite.GTT{}, nil
}
func (f *Fake) OrderMargins(ctx context.Context, p kite.GetMarginParams) ([]kite.OrderMargins, error) {
	f.count("OrderMargins")
	return nil, nil
}
func (f *Fake) BasketMargins(ctx context.Context, p kite.GetBasketParams) (kite.BasketMargins, error) {
	f.count("BasketMargins")
	return kite.BasketMargins{}, nil
}
func (f *Fake) OrderCharges(ctx context.Context, p kite.GetChargesParams) ([]kite.OrderCharges, error) {
	f.count("OrderCharges")
	return nil, nil
}
func (f *Fake) Alerts(ctx context.Context, filters map[string]string) ([]kite.Alert, error) {
	f.count("Alerts")
	return nil, nil
}
func (f *Fake) AlertHistory(ctx context.Context, uuid string) ([]kite.AlertHistory, error) {
	f.count("AlertHistory")
	return nil, nil
}

// Methods returns every method name called so far, sorted (G-0 check helper).
func (f *Fake) Methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for m := range f.calls {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
```

Then make the WebSocket library a direct requirement:

Run: `GOWORK=off go mod tidy`
Expected: `github.com/gorilla/websocket v1.4.2` listed without `// indirect`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOWORK=off go test -tags webkit2_41,noorders -race -short ./internal/kite/mock/`
Expected: `ok  	tradeloom/internal/kite/mock`; no `DATA RACE`.

- [ ] **Step 5: Commit**

```bash
git add internal/kite/mock/packets.go \
  internal/kite/mock/server.go \
  internal/kite/mock/fake.go \
  internal/kite/mock/mock_test.go
git commit -m "$(cat <<'EOF'
test(kite/mock): mock Kite server, tick packet encoder and in-memory read client

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```
