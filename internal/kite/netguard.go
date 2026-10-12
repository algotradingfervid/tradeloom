// internal/kite/netguard.go
package kite

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// AllowedHosts is the NFR-04 network allow-list: the REST API, the login page and the WebSocket feed.
var AllowedHosts = []string{"api.kite.trade", "kite.zerodha.com", "ws.kite.trade"}

// ErrHostNotAllowed is returned (wrapped) for any request to a host outside the allow-list.
var ErrHostNotAllowed = errors.New("kite: host is not on the network allow-list")

type allowListTransport struct {
	next    http.RoundTripper
	allowed map[string]bool
}

// NewAllowListTransport refuses every request whose host is not in allowed before it reaches next.
// cmd/tradeloom passes AllowedHosts; tests pass the mock server's host.
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
	return t.next.RoundTrip(req)
}
