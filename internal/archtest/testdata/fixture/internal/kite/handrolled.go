package kite

import (
	"context"
	"net/http"
	h "net/http"
	"strings"
)

// HandRolled builds its own HTTP client in internal/kite, outside the files allowed to (g0-httpuse): no
// write deny-list sits under it. Every net/http identity below is a violation, one per line.
func HandRolled(ctx context.Context) {
	c := &http.Client{}
	req, _ := http.NewRequest("GET", "https://api.kite.trade/user/profile", strings.NewReader(""))
	_, _ = c.Do(req)
	_, _ = http.DefaultClient.Get("https://api.kite.trade/user/profile")
	_, _ = http.Post("https://api.kite.trade/user/profile", "text/plain", nil)
	_, _ = http.PostForm("https://api.kite.trade/user/profile", nil)
	_, _ = h.Get("https://api.kite.trade/user/profile")
	head := http.Head
	_ = head
	_ = &http.Transport{}
	_ = http.DefaultTransport
	_, _ = http.NewRequestWithContext(ctx, "GET", "https://api.kite.trade/user/profile", nil)
	do := c.Do
	_ = do
	_ = (*http.Client).PostForm
}

// Fine: other net/http identities (a status constant, a handler type).
var _ = http.StatusOK
var _ http.Handler
