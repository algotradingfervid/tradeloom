// internal/kite/netguard.go
package kite

import (
	"errors"
	"fmt"
	"net"
	"net/http"
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

type allowListTransport struct {
	next    http.RoundTripper
	allowed map[string]bool
}

// NewAllowListTransport refuses every request whose host is not in allowed, or whose scheme is not
// https unless the host is loopback (127.0.0.0/8, ::1, localhost: the mock servers of tests), before it
// reaches next. cmd/tradeloom passes AllowedHosts; tests pass the mock server's host.
func NewAllowListTransport(next http.RoundTripper, allowed []string) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	m := make(map[string]bool, len(allowed))
	for _, h := range allowed {
		m[strings.ToLower(h)] = true
	}
	return &allowListTransport{next: next, allowed: m}
}

func (t *allowListTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := strings.ToLower(req.URL.Hostname())
	if !t.allowed[host] {
		return nil, fmt.Errorf("%w: %s", ErrHostNotAllowed, host)
	}
	if !strings.EqualFold(req.URL.Scheme, "https") && !isLoopback(host) {
		return nil, fmt.Errorf("%w: %s requires https", ErrHostNotAllowed, host)
	}
	return t.next.RoundTrip(req)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
