package kite

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ratelimit"
)

// swapDefaultTransport replaces http.DefaultTransport with a recorder for the test, so a client that
// falls back to the default transport never reaches the network.
func swapDefaultTransport(t *testing.T) *recordingNext {
	t.Helper()
	next := &recordingNext{}
	old := http.DefaultTransport
	http.DefaultTransport = rtFunc(func(req *http.Request) (*http.Response, error) {
		_, _ = next.RoundTrip(req)
		body := `{"status":"success","data":{}}`
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	return next
}

// Ruling 83 (1): kite.New with a nil Transport still refuses order writes; the read path still works.
func TestNew_NilTransportRefusesOrderWrite(t *testing.T) {
	next := swapDefaultTransport(t)
	clk := clock.New(clock.Options{})
	lim := ratelimit.New(clk, ratelimit.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lim.Run(ctx)
	c := New(Options{APIKey: "k", BaseURI: "https://api.kite.trade", Limiter: lim, Clock: clk})
	c.SetAccessToken("tok")

	err := c.Call(ctx, ratelimit.ClassOrders, false, "probe", func(kc *kiteconnect.Client) error {
		_, err := kc.PlaceOrder("regular", kiteconnect.OrderParams{Exchange: "NSE", Tradingsymbol: "INFY", Quantity: 1})
		return err
	})
	if err == nil {
		t.Fatal("POST /orders/regular succeeded")
	}
	if r := next.reached(); len(r) != 0 {
		t.Fatalf("order write reached the transport: %v", r)
	}
	// The same client still reaches the (recording) default transport for reads.
	_ = c.Call(ctx, ratelimit.ClassOther, true, "probe", func(kc *kiteconnect.Client) error {
		_, err := kc.GetUserProfile()
		return err
	})
	if r := next.reached(); len(r) == 0 || r[0] != "GET /user/profile" {
		t.Fatalf("read did not reach the default transport: %v", r)
	}
}

// Ruling 83 (1): NewAuth with a caller-supplied client wraps its transport with the write deny-list, and
// leaves the caller's client untouched.
func TestNewAuth_CustomClientRefusesOrderWrite(t *testing.T) {
	next := &recordingNext{}
	hc := &http.Client{Transport: next}
	a := NewAuth(hc, "https://api.kite.trade")("k").(*kcAuth)
	if hc.Transport != http.RoundTripper(next) {
		t.Fatal("NewAuth changed the caller's client")
	}
	if _, err := a.c.PlaceOrder("regular", kiteconnect.OrderParams{Exchange: "NSE", Tradingsymbol: "INFY", Quantity: 1}); err == nil {
		t.Fatal("POST /orders/regular succeeded")
	}
	if r := next.reached(); len(r) != 0 {
		t.Fatalf("order write reached the transport: %v", r)
	}
	_, _ = a.GenerateSession(context.Background(), "rtok", "secret")
	if r := next.reached(); len(r) != 1 || r[0] != "POST /session/token" {
		t.Fatalf("login exchange: reached %v", r)
	}
}

// Ruling 83 (1): NewAuth with a client whose Transport is nil guards http.DefaultTransport.
func TestNewAuth_NilTransportRefusesOrderWrite(t *testing.T) {
	next := swapDefaultTransport(t)
	a := NewAuth(&http.Client{}, "https://api.kite.trade")("k").(*kcAuth)
	if _, err := a.c.PlaceOrder("regular", kiteconnect.OrderParams{}); err == nil || len(next.reached()) != 0 {
		t.Fatalf("err %v, reached %v", err, next.reached())
	}
}

// Ruling 83 (1): wrapping is idempotent; an allow-list transport (which already applies the deny-list) is
// not wrapped again.
func TestGuardWrites_Idempotent(t *testing.T) {
	next := &recordingNext{}
	g := guardWrites(next)
	if _, ok := g.(*writeGuard); !ok {
		t.Fatalf("guardWrites(next) = %T", g)
	}
	if guardWrites(g) != g {
		t.Fatal("guardWrites is not idempotent")
	}
	al := NewAllowListTransport(next, []string{"api.kite.trade"})
	if guardWrites(al) != al {
		t.Fatal("allow-list transport wrapped again")
	}
	if wg, ok := guardWrites(nil).(*writeGuard); !ok || wg.next != http.DefaultTransport {
		t.Fatalf("guardWrites(nil) = %#v", guardWrites(nil))
	}
}

// Ruling 83 (2): any non-empty URL.Opaque is refused on a method other than GET/HEAD, even when it names
// an allowed path; GET/HEAD with an Opaque URL still pass. Both transports apply it.
func TestDenyList_RefusesOpaque(t *testing.T) {
	for _, mk := range []func(http.RoundTripper) http.RoundTripper{
		guardWrites,
		func(n http.RoundTripper) http.RoundTripper {
			return NewAllowListTransport(n, []string{"api.kite.trade"})
		},
	} {
		for _, op := range []string{"/session/token", "//api.kite.trade/session/token", "/margins/orders"} {
			next := &recordingNext{}
			tr := mk(next)
			req := &http.Request{Method: http.MethodPost, Header: http.Header{}, Host: "api.kite.trade",
				URL: &url.URL{Scheme: "https", Host: "api.kite.trade", Opaque: op, Path: "/session/token"}}
			if _, err := tr.RoundTrip(req); !errors.Is(err, ErrRequestNotAllowed) || len(next.reached()) != 0 {
				t.Errorf("%T POST opaque %q: err %v, reached %v", tr, op, err, next.reached())
			}
			get := &http.Request{Method: http.MethodGet, Header: http.Header{}, Host: "api.kite.trade",
				URL: &url.URL{Scheme: "https", Host: "api.kite.trade", Opaque: op}}
			if _, err := tr.RoundTrip(get); err != nil {
				t.Errorf("%T GET opaque %q refused: %v", tr, op, err)
			}
		}
	}
}

// The standalone write guard refuses the same writes as the allow-list transport but has no host check.
func TestWriteGuard_RefusesWritesAnyHost(t *testing.T) {
	next := &recordingNext{}
	tr := guardWrites(next)
	for _, u := range []string{"http://127.0.0.1:1/orders/regular", "https://api.kite.trade/gtt/triggers"} {
		req, _ := http.NewRequest(http.MethodPost, u, nil)
		if _, err := tr.RoundTrip(req); !errors.Is(err, ErrRequestNotAllowed) {
			t.Errorf("%s: %v", u, err)
		}
	}
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:1/session/token", nil)
	if _, err := tr.RoundTrip(req); err != nil {
		t.Errorf("login exchange refused: %v", err)
	}
	if r := next.reached(); len(r) != 1 || r[0] != "POST /session/token" {
		t.Fatalf("reached %v", r)
	}
}
