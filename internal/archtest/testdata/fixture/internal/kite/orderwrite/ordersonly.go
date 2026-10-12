//go:build orders

package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// OrdersOnly is a G-0 violation: linked under -tags orders,noorders.
func OrdersOnly(c *kiteconnect.Client) { c.PlaceOrder("regular", kiteconnect.OrderParams{}) }
