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
