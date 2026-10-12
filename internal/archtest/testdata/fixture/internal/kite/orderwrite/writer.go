//go:build orders && !noorders

package orderwrite

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// Place is allowed: tagged file in internal/kite/orderwrite.
func Place(c *kiteconnect.Client) { c.PlaceOrder("regular", kiteconnect.OrderParams{}) }
