package mock

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/zerodha/gokiteconnect/v4/models"
	kiteticker "github.com/zerodha/gokiteconnect/v4/ticker"

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

// TestFramesParseWithGokiteTicker proves the encoder against the real library's parser by
// pushing every packet shape through a real ticker.Ticker connected to the mock server.
func TestFramesParseWithGokiteTicker(t *testing.T) {
	s := NewServer()
	defer s.Close()
	tk := kiteticker.New("key", "tok")
	tk.SetRootURL(s.TickerURL())
	tk.SetAutoReconnect(false)
	var mu sync.Mutex
	var got []models.Tick
	connected := make(chan struct{}, 1)
	tk.OnConnect(func() { connected <- struct{}{} })
	tk.OnTick(func(tick models.Tick) { mu.Lock(); got = append(got, tick); mu.Unlock() })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { tk.ServeWithContext(ctx); close(done) }()
	defer func() { cancel(); tk.Stop(); s.DropConnections(); <-done }()
	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("no connect")
	}
	ts := time.Date(2026, 10, 12, 10, 0, 1, 0, clock.IST)
	eq := Packet{Token: 738561, Mode: "full", LastPrice: 2450.05, LastQty: 7, AvgPrice: 2449.5, Volume: 1000,
		BuyQty: 11, SellQty: 12, Open: 2400, High: 2460, Low: 2390, Close: 2410.25, LastTradeTime: ts, OI: 55, Timestamp: ts}
	eqQuote := eq
	eqQuote.Mode = "quote"
	idx := Packet{Token: 256265, Mode: "full", LastPrice: 24500.5, Open: 24400, High: 24600, Low: 24300, Close: 24450, Timestamp: ts}
	idxQuote := idx
	idxQuote.Mode = "quote"
	s.PushTicks(eq, eqQuote, idx, idxQuote, Packet{Token: 738561, Mode: "ltp", LastPrice: 1.05})
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n == 5 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("got %d ticks", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	f := got[0]
	if f.Mode != "full" || f.InstrumentToken != 738561 || f.LastPrice != 2450.05 || f.LastTradedQuantity != 7 || f.AverageTradePrice != 2449.5 ||
		f.VolumeTraded != 1000 || f.TotalBuyQuantity != 11 || f.TotalSellQuantity != 12 || f.OHLC.Open != 2400 || f.OHLC.High != 2460 ||
		f.OHLC.Low != 2390 || f.OHLC.Close != 2410.25 || f.OI != 55 || f.Timestamp.Unix() != ts.Unix() || f.LastTradeTime.Unix() != ts.Unix() || !f.IsTradable {
		t.Fatalf("equity full %+v", f)
	}
	if got[1].Mode != "quote" || got[1].LastPrice != 2450.05 || got[1].OHLC.Close != 2410.25 {
		t.Fatalf("equity quote %+v", got[1])
	}
	if i := got[2]; i.Mode != "full" || !i.IsIndex || i.LastPrice != 24500.5 || i.OHLC.High != 24600 || i.OHLC.Open != 24400 || i.OHLC.Close != 24450 || i.Timestamp.Unix() != ts.Unix() {
		t.Fatalf("index full %+v", i)
	}
	if got[3].Mode != "quote" || !got[3].IsIndex || got[3].OHLC.Low != 24300 {
		t.Fatalf("index quote %+v", got[3])
	}
	if got[4].Mode != "ltp" || got[4].LastPrice != 1.05 {
		t.Fatalf("ltp %+v", got[4])
	}
}
