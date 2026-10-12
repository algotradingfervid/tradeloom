// Package kite stands in for tradeloom/internal/kite: Client.Call hands out the raw gokiteconnect client.
package kite

import (
	"context"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

type Client struct{ api *kiteconnect.Client }

func (c *Client) Call(ctx context.Context, op string, fn func(kc *kiteconnect.Client) error) error {
	return fn(c.api)
}

// Session is allowed: Call used inside internal/kite for a read.
func (c *Client) Session(ctx context.Context) error {
	return c.Call(ctx, "session", func(kc *kiteconnect.Client) error { _, err := kc.GetOrders(); return err })
}
