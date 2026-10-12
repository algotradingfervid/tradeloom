// internal/kite/client_key_test.go
package kite

import (
	"context"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite/ratelimit"
)

// SetAPIKey installs the new key for every later attempt (P0-2's Client builds one gokiteconnect client
// per attempt: 10 s api, 30 s hist) on the same recording transport.
func TestSetAPIKey_RebuildsInnerClients(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 10, 12, 9, 0, 0, 0, clock.IST))
	c := New(Options{APIKey: "key1", BaseURI: "http://127.0.0.1:1", Limiter: ratelimit.New(clk, ratelimit.Options{}), Clock: clk})
	rec := c.rec
	c.SetAPIKey("key2")
	if c.APIKey() != "key2" {
		t.Fatalf("APIKey() = %q", c.APIKey())
	}
	for name, class := range map[string]ratelimit.Class{"api": ratelimit.ClassOther, "hist": ratelimit.ClassHistorical} {
		if u := c.attempt(context.Background(), class).GetLoginURL(); !strings.Contains(u, "api_key=key2") {
			t.Fatalf("%s client still uses the old key: %s", name, u)
		}
	}
	if c.rec != rec {
		t.Fatal("the recording transport must survive a key change")
	}
}
