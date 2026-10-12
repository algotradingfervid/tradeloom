package kite

import "net/http"

// Transport is allowed: net/http is imported under internal/kite (g0-http).
var Transport http.RoundTripper = http.DefaultTransport
