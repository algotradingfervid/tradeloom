package kite

import (
	_ "crypto/tls"
	_ "net"
	"net/http"
)

// Transport is allowed: net/http, net and crypto/tls are imported under internal/kite (g0-http, g0-net).
var Transport http.RoundTripper = http.DefaultTransport
