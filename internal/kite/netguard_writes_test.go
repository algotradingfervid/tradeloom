package kite

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// recordingNext stands in for the network: it records what got past the allow-list transport.
type recordingNext struct {
	mu   sync.Mutex
	seen []string
}

func (r *recordingNext) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.seen = append(r.seen, req.Method+" "+req.URL.RequestURI())
	r.mu.Unlock()
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{}, Request: req}, nil
}

func (r *recordingNext) reached() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.seen...)
}

func guardForTest(t *testing.T) (http.RoundTripper, *recordingNext, *bytes.Buffer) {
	t.Helper()
	next := &recordingNext{}
	var logBuf bytes.Buffer
	tr := NewAllowListTransport(next, []string{"api.kite.trade"})
	tr.(*allowListTransport).log = slog.New(slog.NewTextHandler(&logBuf, nil))
	return tr, next, &logBuf
}

// Ruling 79 M-2: the allow-list transport refuses every request that could change account state. Only
// GET/HEAD, POST/DELETE /session/token and the POST margin/charges reads the client uses get through.
func TestAllowListTransport_RefusesWrites(t *testing.T) {
	allowed := []struct{ method, url string }{
		{"GET", "https://api.kite.trade/orders"},
		{"", "https://api.kite.trade/user/profile"}, // "" is GET
		{"HEAD", "https://api.kite.trade/orders"},
		{"get", "https://api.kite.trade/portfolio/holdings"}, // mixed-case method of a read
		{"POST", "https://api.kite.trade/session/token"},
		{"DELETE", "https://api.kite.trade/session/token?api_key=k&access_token=t"},
		{"POST", "https://api.kite.trade/margins/orders?mode=compact"},            // kc.GetOrderMargins
		{"POST", "https://api.kite.trade/margins/basket?consider_positions=true"}, // kc.GetBasketMargins
		{"POST", "https://api.kite.trade/charges/orders"},                         // kc.GetOrderCharges
	}
	for _, c := range allowed {
		tr, next, _ := guardForTest(t)
		req, err := http.NewRequest(c.method, c.url, strings.NewReader("x=1"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tr.RoundTrip(req); err != nil || len(next.reached()) != 1 {
			t.Errorf("%s %s refused: %v", c.method, c.url, err)
		}
	}
	refused := []struct{ method, url, logged string }{
		{"POST", "https://api.kite.trade/orders/regular", "/orders/regular"},
		{"PUT", "https://api.kite.trade/orders/regular/1", "/orders/regular/1"},
		{"DELETE", "https://api.kite.trade/orders/regular/1", "/orders/regular/1"},
		{"POST", "https://api.kite.trade/gtt/triggers", "/gtt/triggers"},
		{"PUT", "https://api.kite.trade/portfolio/positions", "/portfolio/positions"},
		{"POST", "https://api.kite.trade/session/refresh_token", "/session/refresh_token"}, // renew is not used
		{"PUT", "https://api.kite.trade/session/token", "/session/token"},
		{"PATCH", "https://api.kite.trade/session/token", "/session/token"},
		{"OPTIONS", "https://api.kite.trade/orders", "/orders"},
		// Mixed case: the method is folded (still a write) and a path that only folds to an allowed one
		// is refused, because Kite would see the unfolded path.
		{"post", "https://api.kite.trade/orders/regular", "/orders/regular"},
		{"Post", "https://api.kite.trade/Session/Token", "/session/token"},
		{"POST", "https://api.kite.trade/ORDERS/regular", "/orders/regular"},
		// Percent-encoding and dot segments: matched on the cleaned, decoded path, and refused unless the
		// path on the wire is already that canonical path.
		{"POST", "https://api.kite.trade/%6Frders/regular", "/orders/regular"},
		{"POST", "https://api.kite.trade/session/%74oken", "/session/token"},
		{"POST", "https://api.kite.trade/session/token%2F..%2F..%2Forders%2Fregular", "/orders/regular"},
		{"POST", "https://api.kite.trade/session/token/../../orders/regular", "/orders/regular"},
		{"POST", "https://api.kite.trade/orders/regular/../../session/token", "/session/token"},
		{"POST", "https://api.kite.trade/session/token/", "/session/token"},
		{"POST", "https://api.kite.trade//session/token", "/session/token"},
		// A base URI ending in "#" or "?" turns the client's "/session/token" into a fragment or query.
		{"POST", "https://api.kite.trade/orders/regular#/session/token", "/orders/regular"},
		{"POST", "https://api.kite.trade/orders/regular?/session/token", "/orders/regular"},
	}
	for _, c := range refused {
		tr, next, logBuf := guardForTest(t)
		req, err := http.NewRequest(c.method, c.url, strings.NewReader("tradingsymbol=INFY&secretbody=1"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = tr.RoundTrip(req)
		if !errors.Is(err, ErrRequestNotAllowed) || len(next.reached()) != 0 {
			t.Errorf("%s %s: err %v, reached %v", c.method, c.url, err, next.reached())
			continue
		}
		log := logBuf.String()
		if !strings.Contains(log, "path="+c.logged) || !strings.Contains(err.Error(), c.logged) {
			t.Errorf("%s %s: refusal does not name %s: err %q, log %q", c.method, c.url, c.logged, err, log)
		}
		for _, leak := range []string{"secretbody", "INFY", "?/session"} {
			if strings.Contains(log, leak) || strings.Contains(err.Error(), leak) {
				t.Errorf("%s %s: refusal leaks %q: err %q, log %q", c.method, c.url, leak, err, log)
			}
		}
	}
}

// A hand-built request whose request line differs from URL.Path: Opaque wins on the wire, and a raw
// CR/LF in the path is escaped, so neither can pass as /session/token.
func TestAllowListTransport_RequestLine(t *testing.T) {
	for name, u := range map[string]*url.URL{
		"opaque path":      {Scheme: "https", Host: "api.kite.trade", Opaque: "/orders/regular", Path: "/session/token"},
		"opaque authority": {Scheme: "https", Host: "api.kite.trade", Opaque: "//api.kite.trade/orders/regular", Path: "/session/token"},
		"CRLF in path":     {Scheme: "https", Host: "api.kite.trade", Path: "/session/token HTTP/1.1\r\nHost: api.kite.trade\r\n\r\nPOST /orders/regular"},
	} {
		tr, next, _ := guardForTest(t)
		req := &http.Request{Method: http.MethodPost, URL: u, Header: http.Header{}, Host: "api.kite.trade"}
		if _, err := tr.RoundTrip(req); !errors.Is(err, ErrRequestNotAllowed) || len(next.reached()) != 0 {
			t.Errorf("%s: err %v, reached %v", name, err, next.reached())
		}
	}
}

// gokiteconnect builds every URL as baseURI+path, so SetBaseURI("…/orders/regular#") would turn the
// login exchange into POST /orders/regular. The transport refuses it before the network.
func TestAllowListTransport_SetBaseURITricks(t *testing.T) {
	for _, base := range []string{"https://api.kite.trade/orders/regular#", "https://api.kite.trade/orders/regular?", "https://api.kite.trade/orders/regular/%2E%2E"} {
		tr, next, logBuf := guardForTest(t)
		kc := kiteconnect.New("k")
		kc.SetHTTPClient(&http.Client{Transport: tr})
		kc.SetBaseURI(base)
		if _, err := kc.GenerateSession("rtok", "s3cr3t"); err == nil {
			t.Errorf("%s: login exchange succeeded", base)
		}
		if r := next.reached(); len(r) != 0 {
			t.Errorf("%s: reached the network: %v", base, r)
		}
		if !strings.Contains(logBuf.String(), "kite request refused") || strings.Contains(logBuf.String(), "s3cr3t") {
			t.Errorf("%s: log %q", base, logBuf.String())
		}
	}
}
