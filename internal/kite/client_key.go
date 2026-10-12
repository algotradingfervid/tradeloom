// internal/kite/client_key.go
package kite

// SetAPIKey installs a new API key (Settings → Kite connection). P0-2's Client builds a fresh
// gokiteconnect client per attempt (10 s api, 30 s hist) on the one recording transport, so
// changing c.o.APIKey under c.mu is enough: every later attempt uses the new key and the same
// recorder. The Session always calls it before SetAccessToken.
func (c *Client) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.o.APIKey = key
}
