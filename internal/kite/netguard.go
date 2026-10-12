// internal/kite/netguard.go
package kite

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// AllowedHosts is the NFR-04 network allow-list: the REST API, the login page and the WebSocket feed.
//
// Scope: NewAllowListTransport guards HTTP(S) requests only (the REST client and the session calls of
// AuthAPI). The ticker WebSocket (ticker.KiteDialer) dials through gorilla/websocket and does NOT pass
// through this transport; it reaches only the ws.kite.trade root it is given (or the mock root in tests).
var AllowedHosts = []string{"api.kite.trade", "kite.zerodha.com", "ws.kite.trade"}

// ErrHostNotAllowed is returned (wrapped) for any request to a host outside the allow-list, and for a
// plain-http request to any host that is not a loopback test host.
var ErrHostNotAllowed = errors.New("kite: host is not on the network allow-list")

// ErrRequestNotAllowed is returned (wrapped) for a request that could change account state.
var ErrRequestNotAllowed = errors.New("kite: request is not on the read-only allow-list")

type allowListTransport struct {
	next    http.RoundTripper
	allowed map[string]bool
	log     *slog.Logger
}

// NewAllowListTransport refuses every request whose host is not in allowed, or whose scheme is not
// https unless the host is loopback (127.0.0.0/8, ::1, localhost: the mock servers of tests), before it
// reaches next. cmd/tradeloom passes AllowedHosts; tests pass the mock server's host. It also refuses
// (ErrRequestNotAllowed, logged without query or body) every request other than GET/HEAD that is not
// in writeAllowed (Ruling 79 M-2).
func NewAllowListTransport(next http.RoundTripper, allowed []string) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	m := make(map[string]bool, len(allowed))
	for _, h := range allowed {
		m[strings.ToLower(h)] = true
	}
	return &allowListTransport{next: next, allowed: m, log: slog.Default()}
}

// writeAllowed are the only requests other than GET/HEAD the app sends to Kite (Ruling 79 M-2, defence
// in depth for G-0): the login exchange and logout, and the three reads gokiteconnect sends as POST
// with a JSON body (kite.Client.OrderMargins, BasketMargins and OrderCharges). Every other method/path
// pair is refused before it reaches the network, whatever built the request.
var writeAllowed = map[string]bool{
	"POST /session/token":   true, // GenerateSession
	"DELETE /session/token": true, // InvalidateAccessToken
	"POST /margins/orders":  true, // GetOrderMargins
	"POST /margins/basket":  true, // GetBasketMargins
	"POST /charges/orders":  true, // GetOrderCharges
}

func (t *allowListTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := strings.ToLower(req.URL.Hostname())
	if !t.allowed[host] {
		return nil, fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
	}
	if !strings.EqualFold(req.URL.Scheme, "https") && !isLoopback(host) {
		return nil, fmt.Errorf("%w: %s requires https", ErrHostNotAllowed, host)
	}
	method := strings.ToUpper(req.Method)
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodHead {
		wire, canon := requestPaths(req.URL)
		// Match on the canonical path, and require the path on the wire to be exactly that: an encoded,
		// dotted, doubled-slash or upper-case variant could reach a different route at Kite.
		if !writeAllowed[method+" "+canon] || wire != canon {
			// Never log the query or the body: they can carry the API secret, the checksum or a token.
			t.log.Warn("kite request refused: not on the read-only allow-list", "method", method, "host", host, "path", canon)
			return nil, fmt.Errorf("%w: %s %s", ErrRequestNotAllowed, method, canon)
		}
	}
	return t.next.RoundTrip(req)
}

// requestPaths returns the path as it goes on the request line (from URL.RequestURI, so URL.Opaque wins
// over URL.Path; the query and fragment are dropped) and its canonical form: percent-decoded until
// stable, cleaned of dot segments and repeated slashes, and lower-cased.
func requestPaths(u *url.URL) (wire, canon string) {
	ru := u.RequestURI()
	if i := strings.IndexByte(ru, '?'); i >= 0 {
		ru = ru[:i]
	}
	if !strings.HasPrefix(ru, "/") { // absolute form ("https://host/path") from an Opaque "//host/path"
		if i := strings.Index(ru, "://"); i >= 0 {
			ru = ru[i+3:]
		}
		if j := strings.IndexByte(ru, '/'); j >= 0 {
			ru = ru[j:]
		} else {
			ru = "/"
		}
	}
	dec := ru
	for i := 0; i < 4; i++ {
		d, err := url.PathUnescape(dec)
		if err != nil || d == dec {
			break
		}
		dec = d
	}
	return ru, strings.ToLower(path.Clean("/" + dec))
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
