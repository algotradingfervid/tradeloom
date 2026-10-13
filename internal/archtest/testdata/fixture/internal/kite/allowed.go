package kite

import "net/http"

// Shared is allowed by the test allow-list (net/http.Client in internal/kite/allowed.go): no violation.
func Shared() *http.Client { return &http.Client{} }
